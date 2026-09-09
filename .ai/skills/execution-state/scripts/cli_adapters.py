#!/usr/bin/env python3
"""Capability-based, vendor-neutral CLI launch planning.

This module never launches an agent.  It only discovers declared runtimes and
builds an argv-based handoff plan that a trusted controller may inspect and
execute.  Agent prompts remain external files.
"""

from __future__ import annotations

import json
import os
import re
import shutil
import string
import subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Iterable


MANIFEST_VERSION = "1.0.0"
STDIN_PROTOCOL = "execution-state.stdin/1.0.0"
CAPABILITY_KEYS = {
    "fresh_context",
    "in_place_compaction",
    "machine_output",
    "session_persistence_control",
    "usage_metrics",
}
RESET_CAPABILITIES = {"fresh_context", "in_place_compaction"}
ALLOWED_PLACEHOLDERS = {
    "executable",
    "prompt_path",
    "packet_path",
    "project_root",
}
BUILTIN_IDS = ("manual", "codex", "claude", "gemini")
_ID_RE = re.compile(r"^[a-z][a-z0-9-]{0,63}$")
_PERMISSION_BYPASS_TOKENS = {
    "--dangerously-skip-permissions",
    "--yolo",
}
_SHELL_EXECUTABLES = {
    "bash",
    "cmd",
    "cmd.exe",
    "env",
    "fish",
    "powershell",
    "powershell.exe",
    "pwsh",
    "sh",
    "zsh",
}


class AdapterError(ValueError):
    """Raised for an invalid adapter manifest or launch plan."""


@dataclass(frozen=True)
class AdapterManifest:
    """Validated declarative adapter definition."""

    adapter_id: str
    executables: tuple[str, ...]
    capabilities: dict[str, bool]
    launch: dict[str, Any] | None
    origin: str


def _adapter_dir() -> Path:
    return Path(__file__).resolve().parent / "adapters"


def _read_json(path: Path) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise AdapterError(f"adapter manifest not found: {path}") from exc
    except json.JSONDecodeError as exc:
        raise AdapterError(f"invalid adapter JSON in {path}: {exc}") from exc
    if not isinstance(value, dict):
        raise AdapterError(f"adapter manifest must be an object: {path}")
    return value


def _validate_template(value: str, field: str) -> None:
    if not isinstance(value, str) or not value or "\x00" in value:
        raise AdapterError(f"{field} must be a non-empty string without NUL")
    try:
        parsed = list(string.Formatter().parse(value))
    except ValueError as exc:
        raise AdapterError(f"invalid template in {field}: {exc}") from exc
    for _, name, format_spec, conversion in parsed:
        if name is None:
            continue
        if name not in ALLOWED_PLACEHOLDERS:
            raise AdapterError(f"unsupported placeholder {{{name}}} in {field}")
        if format_spec or conversion:
            raise AdapterError(f"format modifiers are not allowed in {field}")
        if value != "{" + name + "}":
            raise AdapterError(f"placeholder in {field} must occupy the whole argv element")


def _reject_permission_bypass(value: str, field: str) -> None:
    normalized = value.strip().lower()
    if (
        normalized in _PERMISSION_BYPASS_TOKENS
        or "dangerously-bypass" in normalized
    ):
        raise AdapterError(f"permission-bypass option is not allowed in {field}")


def _validate_manifest(data: dict[str, Any], origin: str) -> AdapterManifest:
    allowed = {"schema_version", "id", "executables", "capabilities", "launch"}
    extra = sorted(set(data) - allowed)
    if extra:
        raise AdapterError(f"unsupported manifest fields: {', '.join(extra)}")
    if data.get("schema_version") != MANIFEST_VERSION:
        raise AdapterError(f"schema_version must equal {MANIFEST_VERSION}")

    adapter_id = data.get("id")
    if not isinstance(adapter_id, str) or not _ID_RE.fullmatch(adapter_id):
        raise AdapterError("adapter id must match [a-z][a-z0-9-]{0,63}")

    raw_executables = data.get("executables")
    if not isinstance(raw_executables, list) or not all(
        isinstance(item, str)
        and item
        and "\x00" not in item
        and "\n" not in item
        and "\r" not in item
        and not item.startswith("-")
        for item in raw_executables
    ):
        raise AdapterError("executables must be a list of safe non-empty names or paths")
    if adapter_id != "manual" and not raw_executables:
        raise AdapterError("non-manual adapters need at least one executable")
    for executable in raw_executables:
        if Path(executable).name.lower() in _SHELL_EXECUTABLES:
            raise AdapterError("shell and env executables are not allowed in adapter manifests")

    capabilities = data.get("capabilities")
    if not isinstance(capabilities, dict) or set(capabilities) != CAPABILITY_KEYS:
        raise AdapterError(
            "capabilities must contain exactly: " + ", ".join(sorted(CAPABILITY_KEYS))
        )
    if not all(isinstance(value, bool) for value in capabilities.values()):
        raise AdapterError("all capabilities must be boolean")

    launch = data.get("launch")
    if adapter_id == "manual":
        if launch is not None:
            raise AdapterError("manual adapter launch must be null")
        if any(capabilities.values()):
            raise AdapterError("manual adapter cannot confirm runtime capabilities")
    else:
        if not isinstance(launch, dict):
            raise AdapterError("non-manual adapter launch must be an object")
        launch_allowed = {"strategy", "argv", "stdin_files"}
        launch_extra = sorted(set(launch) - launch_allowed)
        if launch_extra:
            raise AdapterError(f"unsupported launch fields: {', '.join(launch_extra)}")
        if launch.get("strategy") not in {"fresh_process", "in_place_compaction"}:
            raise AdapterError("launch.strategy is unsupported")
        argv = launch.get("argv")
        if not isinstance(argv, list) or not argv or not all(
            isinstance(item, str) for item in argv
        ):
            raise AdapterError("launch.argv must be a non-empty string list")
        if argv[0] != "{executable}":
            raise AdapterError("launch.argv[0] must be {executable}")
        for index, item in enumerate(argv):
            _validate_template(item, f"launch.argv[{index}]")
            _reject_permission_bypass(item, f"launch.argv[{index}]")
        stdin_files = launch.get("stdin_files", [])
        if not isinstance(stdin_files, list) or not all(
            isinstance(item, str) for item in stdin_files
        ):
            raise AdapterError("launch.stdin_files must be a string list")
        for index, item in enumerate(stdin_files):
            _validate_template(item, f"launch.stdin_files[{index}]")
        if stdin_files != ["{prompt_path}", "{packet_path}"]:
            raise AdapterError(
                "launch.stdin_files must frame prompt_path then packet_path exactly once"
            )

        strategy_capability = {
            "fresh_process": "fresh_context",
            "in_place_compaction": "in_place_compaction",
        }[launch["strategy"]]
        if not capabilities[strategy_capability]:
            raise AdapterError(
                f"launch strategy requires capability {strategy_capability}=true"
            )

    return AdapterManifest(
        adapter_id=adapter_id,
        executables=tuple(raw_executables),
        capabilities=dict(capabilities),
        launch=launch,
        origin=origin,
    )


def load_manifest(path: str | Path) -> AdapterManifest:
    """Load and validate one custom adapter manifest."""

    manifest_path = Path(path).expanduser().resolve()
    return _validate_manifest(_read_json(manifest_path), str(manifest_path))


def builtin_manifests() -> dict[str, AdapterManifest]:
    """Return the trusted built-in manifests keyed by adapter id."""

    result: dict[str, AdapterManifest] = {}
    for adapter_id in BUILTIN_IDS:
        path = _adapter_dir() / f"{adapter_id}.json"
        manifest = _validate_manifest(_read_json(path), f"builtin:{adapter_id}")
        if manifest.adapter_id != adapter_id:
            raise AdapterError(f"built-in filename/id mismatch: {path}")
        result[adapter_id] = manifest
    return result


def available_manifests(custom_manifest: str | Path | None = None) -> dict[str, AdapterManifest]:
    """Return built-ins plus at most one non-shadowing custom manifest."""

    manifests = builtin_manifests()
    if custom_manifest is not None:
        custom = load_manifest(custom_manifest)
        if custom.adapter_id in manifests:
            raise AdapterError(f"custom adapter cannot replace built-in {custom.adapter_id!r}")
        manifests[custom.adapter_id] = custom
    return manifests


def _resolve_executable(candidates: Iterable[str], search_path: str | None) -> str | None:
    for candidate in candidates:
        candidate_path = Path(candidate).expanduser()
        if candidate_path.is_absolute() or os.sep in candidate or (
            os.altsep and os.altsep in candidate
        ):
            resolved = candidate_path.resolve()
            if resolved.is_file() and os.access(resolved, os.X_OK):
                return str(resolved)
            continue
        found = shutil.which(candidate, path=search_path)
        if found:
            return str(Path(found).resolve())
    return None


def _probe_version(executable: str) -> tuple[str | None, str | None]:
    """Perform a bounded, non-model version handshake."""

    try:
        completed = subprocess.run(
            [executable, "--version"],
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            timeout=2,
            check=False,
            shell=False,
        )
    except (OSError, subprocess.SubprocessError) as exc:
        return None, f"version probe failed: {exc}"
    output = " ".join(completed.stdout.split())[:256]
    if completed.returncode != 0:
        return None, f"version probe exited {completed.returncode}"
    if not output:
        return None, "version probe returned no version"
    return output, None


def _public_candidate(
    manifest: AdapterManifest, executable: str, version: str
) -> dict[str, Any]:
    return {
        "id": manifest.adapter_id,
        "executable": executable,
        "version": version,
        "capabilities": dict(manifest.capabilities),
    }


def probe_runtime(
    requested: str = "auto",
    *,
    custom_manifest: str | Path | None = None,
    search_path: str | None = None,
    trust_custom: bool = False,
) -> dict[str, Any]:
    """Select a runtime by confirmed capabilities, falling back to manual.

    Automatic selection is intentionally conservative: zero or multiple
    discovered agent CLIs are not guessed at.
    """

    manifests = available_manifests(custom_manifest)
    selection_source = "argument"
    invalid_environment: str | None = None
    if requested == "auto":
        environment_adapter = os.environ.get("EXECUTION_STATE_ADAPTER")
        if environment_adapter:
            selection_source = "environment"
            if not _ID_RE.fullmatch(environment_adapter):
                invalid_environment = environment_adapter
            else:
                requested = environment_adapter
        else:
            selection_source = "auto-discovery"
    discovered: list[dict[str, Any]] = []
    unavailable: list[dict[str, str]] = []
    resolved: dict[str, str] = {}
    versions: dict[str, str] = {}
    if invalid_environment is not None or requested == "manual" or requested not in manifests and requested != "auto":
        probe_ids: list[str] = []
    elif requested == "auto":
        probe_ids = [adapter_id for adapter_id in manifests if adapter_id != "manual"]
    else:
        probe_ids = [requested]
    for adapter_id in probe_ids:
        manifest = manifests[adapter_id]
        if not manifest.origin.startswith("builtin:") and not trust_custom:
            unavailable.append(
                {
                    "id": adapter_id,
                    "reason": "custom adapter executable requires explicit trust",
                }
            )
            continue
        executable = _resolve_executable(manifest.executables, search_path)
        if executable:
            version, failure = _probe_version(executable)
            if version is None:
                unavailable.append({"id": adapter_id, "reason": failure or "version probe failed"})
                continue
            resolved[adapter_id] = executable
            versions[adapter_id] = version
            discovered.append(_public_candidate(manifest, executable, version))

    if invalid_environment is not None:
        selected = {
            "id": "manual",
            "executable": None,
            "version": None,
            "capabilities": dict(manifests["manual"].capabilities),
        }
        reason = "EXECUTION_STATE_ADAPTER is not a valid adapter id"
    elif requested == "auto":
        if len(discovered) == 1:
            selected = discovered[0]
            reason = "one compatible runtime discovered"
        elif not discovered:
            selected = {
                "id": "manual",
                "executable": None,
                "version": None,
                "capabilities": dict(manifests["manual"].capabilities),
            }
            reason = "no compatible runtime discovered"
        else:
            selected = {
                "id": "manual",
                "executable": None,
                "version": None,
                "capabilities": dict(manifests["manual"].capabilities),
            }
            reason = "multiple runtimes discovered; choose one explicitly"
    elif requested in manifests and requested != "manual" and requested in resolved:
        manifest = manifests[requested]
        selected = _public_candidate(manifest, resolved[requested], versions[requested])
        reason = "requested runtime discovered"
    elif requested == "manual":
        selected = {
            "id": "manual",
            "executable": None,
            "version": None,
            "capabilities": dict(manifests["manual"].capabilities),
        }
        reason = "manual adapter requested"
    elif requested in manifests:
        selected = {
            "id": "manual",
            "executable": None,
            "version": None,
            "capabilities": dict(manifests["manual"].capabilities),
        }
        reason = f"requested runtime {requested!r} is not available"
    else:
        selected = {
            "id": "manual",
            "executable": None,
            "version": None,
            "capabilities": dict(manifests["manual"].capabilities),
        }
        reason = f"unknown runtime {requested!r}"

    return {
        "selected": selected,
        "reason": reason,
        "selection_source": selection_source,
        "discovered": discovered,
        "unavailable": unavailable,
    }


def supports_reset(probe: dict[str, Any]) -> bool:
    """Return true only for a selected runtime with a confirmed reset ability."""

    capabilities = probe.get("selected", {}).get("capabilities", {})
    return any(capabilities.get(name) is True for name in RESET_CAPABILITIES)


def _render(template: str, values: dict[str, str], field: str) -> str:
    _validate_template(template, field)
    try:
        return template.format_map(values)
    except KeyError as exc:  # defensive; validation normally prevents this
        raise AdapterError(f"missing value for {exc.args[0]!r} in {field}") from exc


def build_runtime_plan(
    *,
    requested: str,
    prompt_path: str | Path,
    packet_path: str | Path,
    project_root: str | Path,
    custom_manifest: str | Path | None = None,
    search_path: str | None = None,
    trust_custom: bool = False,
) -> dict[str, Any]:
    """Build, but never execute, a safe argv-based runtime handoff plan."""

    prompt = Path(prompt_path).expanduser().resolve()
    packet = Path(packet_path).expanduser().resolve()
    root = Path(project_root).expanduser().resolve()
    for label, path in (("prompt", prompt), ("packet", packet)):
        if not path.is_file():
            raise AdapterError(f"{label} file not found: {path}")
    if not root.is_dir():
        raise AdapterError(f"project root not found: {root}")

    probe = probe_runtime(
        requested,
        custom_manifest=custom_manifest,
        search_path=search_path,
        trust_custom=trust_custom,
    )
    selected = probe["selected"]
    adapter_id = selected["id"]
    if adapter_id == "manual":
        return {
            "adapter": "manual",
            "strategy": "manual_handoff",
            "capabilities": selected["capabilities"],
            "prompt_path": str(prompt),
            "packet_path": str(packet),
            "project_root": str(root),
            "reason": probe["reason"],
            "executes": False,
        }

    manifest = available_manifests(custom_manifest)[adapter_id]
    if manifest.launch is None:  # pragma: no cover - prevented by validation
        raise AdapterError(f"adapter {adapter_id!r} has no launch definition")
    values = {
        "executable": selected["executable"],
        "prompt_path": str(prompt),
        "packet_path": str(packet),
        "project_root": str(root),
    }
    argv = [
        _render(item, values, f"launch.argv[{index}]")
        for index, item in enumerate(manifest.launch["argv"])
    ]
    stdin_files = [
        _render(item, values, f"launch.stdin_files[{index}]")
        for index, item in enumerate(manifest.launch.get("stdin_files", []))
    ]
    return {
        "adapter": adapter_id,
        "strategy": manifest.launch["strategy"],
        "capabilities": selected["capabilities"],
        "argv": argv,
        "cwd": str(root),
        "stdin_files": stdin_files,
        "stdin": {
            "protocol": STDIN_PROTOCOL,
            "encoding": "utf-8",
            "parts": [
                {
                    "name": "worker_prompt",
                    "path": stdin_files[0],
                    "begin": "---EXECUTION_STATE_WORKER_PROMPT_1_0_0---",
                    "end": "---END_EXECUTION_STATE_WORKER_PROMPT_1_0_0---",
                },
                {
                    "name": "worker_packet",
                    "path": stdin_files[1],
                    "begin": "---EXECUTION_STATE_WORKER_PACKET_1_0_0---",
                    "end": "---END_EXECUTION_STATE_WORKER_PACKET_1_0_0---",
                },
            ],
            "line_separator": "LF",
        },
        "executes": False,
    }
