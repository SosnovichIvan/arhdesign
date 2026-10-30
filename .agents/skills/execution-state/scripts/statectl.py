#!/usr/bin/env python3
"""Vendor-neutral execution-state 1.0.0 controller.

The controller owns compact state transitions.  It does not contain prompts,
run agent CLIs, clear a model context, or interpret implementation references.
"""

from __future__ import annotations

import argparse
import copy
import errno
import hashlib
import json
import os
import re
import shutil
import sys
import tempfile
import time
import uuid
from contextlib import ExitStack, contextmanager
from pathlib import Path
from typing import Any, Iterable

try:  # POSIX locks are released by the kernel when a process exits.
    import fcntl
except ImportError:  # pragma: no cover - exercised only on non-POSIX hosts
    fcntl = None  # type: ignore[assignment]

from cli_adapters import (
    MANIFEST_VERSION,
    STDIN_PROTOCOL,
    AdapterError,
    build_runtime_plan,
    probe_runtime,
    supports_reset,
)


SCHEMA_VERSION = "1.0.0"
TASK_LEDGER_VERSION = "1.0.0"
PACKET_VERSION = "1.0.0"
WORKER_PROTOCOL = "execution-state.worker/1.0.0"
RESULT_PROTOCOL = "execution-state.result/1.0.0"
SKILL_ROOT = Path(__file__).resolve().parents[1]
RELEASE_MANIFEST = SKILL_ROOT / "release.json"
CONTEXT_MAP_VERSION = "1.0.0"
MAX_STATE_BYTES = 8 * 1024
MAX_PACKET_BYTES = 12 * 1024
MAX_OBSERVATION_BYTES = 2 * 1024
MAX_TASK_LEDGER_BYTES = 64 * 1024
MAX_EVIDENCE_ITEM_BYTES = 1024
MAX_EVIDENCE_BYTES = 4 * 1024
MAX_STRING_BYTES = 4 * 1024
MAX_REVIEW_INPUT_BYTES = 2 * 1024
MAX_REVIEW_RECORD_BYTES = 1024
MAX_REVIEW_EVIDENCE_RECORD_BYTES = 256
ARCHITECTURE_REVIEW_PROTOCOL = "execution-state.review/1.0.0"
STATE_STATUSES = {"planned", "in_progress", "blocked", "complete"}
TASK_STATUSES = {"pending", "in_progress", "blocked", "complete"}
SOURCE_KINDS = {"standalone", "openspec"}
EXECUTION_PROFILES = {"lite", "reset"}
TASK_KINDS = {"implementation", "integration", "architecture", "verification"}
ERROR_INVALID = 3
ERROR_CONFLICT = 4
ERROR_RUNTIME = 5
LOCK_DIRECTORY_STALE_SECONDS = 30
TRANSACTION_VERSION = "1.0.0"
TRANSACTION_MARKER = ".statectl.transaction.json"
TRANSACTION_BACKUP = ".statectl.transaction.authority.bak"
_BANNED_STATE_KEYS = {
    "chain_of_thought",
    "history",
    "messages",
    "raw_log",
    "raw_logs",
    "reasoning",
    "transcript",
}
_TASK_LINE = re.compile(
    r"^(?P<prefix>\s*[-*]\s+\[)(?P<mark>[ xX])(?P<middle>\]\s+)"
    r"(?P<id>[\w]+(?:[._-][\w]+)*)(?P<punct>[.)]?)(?P<space>\s+)"
    r"(?P<title>.+?)\s*$"
)


class StateCtlError(ValueError):
    """A stable, machine-readable controller failure."""

    def __init__(self, message: str, *, kind: str = "invalid_state", code: int = ERROR_INVALID):
        super().__init__(message)
        self.kind = kind
        self.code = code


def _release_info() -> dict[str, Any]:
    release = _read_json(RELEASE_MANIFEST, "release manifest")
    expected = {
        "name": "execution-state",
        "state_schema": SCHEMA_VERSION,
        "task_ledger_schema": TASK_LEDGER_VERSION,
        "context_map_schema": CONTEXT_MAP_VERSION,
        "worker_protocol": WORKER_PROTOCOL,
        "result_protocol": RESULT_PROTOCOL,
        "review_protocol": ARCHITECTURE_REVIEW_PROTOCOL,
        "stdin_protocol": STDIN_PROTOCOL,
        "packet_version": PACKET_VERSION,
        "adapter_manifest_schema": MANIFEST_VERSION,
        "minimum_python": "3.9",
    }
    for field, value in expected.items():
        if release.get(field) != value:
            raise StateCtlError(f"release manifest {field} does not match runtime")
    version = release.get("version")
    if not isinstance(version, str) or not re.fullmatch(
        r"(?:0|[1-9]\d*)(?:\.(?:0|[1-9]\d*)){2}(?:-[0-9A-Za-z.-]+)?",
        version,
    ):
        raise StateCtlError("release manifest version is not valid SemVer")
    if set(release) != {*expected, "version"}:
        raise StateCtlError("release manifest fields are invalid")
    return release


class JsonArgumentParser(argparse.ArgumentParser):
    def error(self, message: str) -> None:
        raise StateCtlError(message, kind="usage", code=2)


def _json_bytes(value: Any) -> bytes:
    return (json.dumps(value, ensure_ascii=False, separators=(",", ":")) + "\n").encode(
        "utf-8"
    )


def _emit(value: dict[str, Any]) -> None:
    sys.stdout.write(_json_bytes(value).decode("utf-8"))


def _byte_len(value: str) -> int:
    return len(value.encode("utf-8"))


def _task_next_action(task: dict[str, Any]) -> str:
    return f"Execute task {task['id']}"


def _truncate_utf8(value: str, max_bytes: int) -> str:
    encoded = value.encode("utf-8")
    if len(encoded) <= max_bytes:
        return value
    return encoded[:max_bytes].decode("utf-8", errors="ignore").rstrip()


def _require_text(
    value: Any,
    field: str,
    *,
    max_bytes: int = MAX_STRING_BYTES,
) -> str:
    if not isinstance(value, str) or not value.strip():
        raise StateCtlError(f"{field} must be a non-empty string")
    if "\x00" in value:
        raise StateCtlError(f"{field} must not contain NUL")
    if _byte_len(value) > max_bytes:
        raise StateCtlError(f"{field} exceeds {max_bytes} bytes")
    return value.strip()


def _safe_id(value: Any, field: str = "id") -> str:
    identifier = _require_text(value, field, max_bytes=128)
    if identifier in {".", ".."} or any(
        separator and separator in identifier
        for separator in ("/", "\\", os.sep, os.altsep)
    ):
        raise StateCtlError(f"{field} must not contain a path separator")
    if any(ord(character) < 32 for character in identifier):
        raise StateCtlError(f"{field} must not contain control characters")
    return identifier


def _string_list(
    value: Any,
    field: str,
    *,
    max_items: int = 32,
    item_bytes: int = MAX_STRING_BYTES,
) -> list[str]:
    if not isinstance(value, list) or len(value) > max_items:
        raise StateCtlError(f"{field} must be a list with at most {max_items} items")
    return [
        _require_text(item, f"{field}[{index}]", max_bytes=item_bytes)
        for index, item in enumerate(value)
    ]


def _evidence_list(value: Any, field: str = "evidence") -> list[str]:
    evidence = _string_list(
        value,
        field,
        max_items=16,
        item_bytes=MAX_EVIDENCE_ITEM_BYTES,
    )
    if sum(_byte_len(item) for item in evidence) > MAX_EVIDENCE_BYTES:
        raise StateCtlError(f"{field} exceeds {MAX_EVIDENCE_BYTES} bytes in total")
    return evidence


def _check_list(value: Any, field: str = "checks") -> list[dict[str, str]]:
    if not isinstance(value, list) or len(value) > 16:
        raise StateCtlError(f"{field} must be a list with at most 16 items")
    result: list[dict[str, str]] = []
    seen: set[str] = set()
    for index, item in enumerate(value):
        if not isinstance(item, dict) or set(item) != {"id", "status", "summary"}:
            raise StateCtlError(f"{field}[{index}] fields are invalid")
        check_id = _safe_id(item.get("id"), f"{field}[{index}].id")
        if check_id in seen:
            raise StateCtlError(f"duplicate check id in {field}: {check_id}")
        seen.add(check_id)
        status = item.get("status")
        if status not in {"passed", "failed"}:
            raise StateCtlError(f"{field}[{index}].status must be passed or failed")
        summary = _require_text(item.get("summary"), f"{field}[{index}].summary", max_bytes=512)
        result.append({"id": check_id, "status": status, "summary": summary})
    return result


def _parse_checks(values: list[str]) -> list[dict[str, str]]:
    checks: list[dict[str, str]] = []
    for value in values:
        try:
            item = json.loads(value)
        except json.JSONDecodeError as exc:
            raise StateCtlError(f"invalid --check-json: {exc}") from exc
        checks.append(item)
    return _check_list(checks)


def _parse_architecture_review(value: str) -> dict[str, Any]:
    try:
        review = json.loads(value)
    except json.JSONDecodeError as exc:
        raise StateCtlError(f"invalid --review-json: {exc}") from exc
    expected = {
        "protocol", "verdict", "summary", "blockers", "planned_gaps",
        "recommendations", "checks",
    }
    if not isinstance(review, dict) or set(review) != expected:
        raise StateCtlError("architecture review fields are invalid")
    if review.get("protocol") != ARCHITECTURE_REVIEW_PROTOCOL:
        raise StateCtlError(
            f"architecture review protocol must equal {ARCHITECTURE_REVIEW_PROTOCOL}"
        )
    verdict = review.get("verdict")
    if verdict not in {"passed", "blocked"}:
        raise StateCtlError("architecture review verdict must be passed or blocked")
    summary = _require_text(
        review.get("summary"),
        "architecture review summary",
        max_bytes=MAX_REVIEW_INPUT_BYTES,
    )
    blockers = review.get("blockers")
    if not isinstance(blockers, list) or len(blockers) > 4:
        raise StateCtlError("architecture review blockers must contain at most 4 items")
    normalized_blockers: list[dict[str, Any]] = []
    for index, blocker in enumerate(blockers):
        if not isinstance(blocker, dict) or set(blocker) != {
            "contract", "evidence", "affected_files", "regression_check",
        }:
            raise StateCtlError(f"architecture review blockers[{index}] fields are invalid")
        normalized_blockers.append(
            {
                "contract": _require_text(
                    blocker.get("contract"),
                    f"architecture review blockers[{index}].contract",
                    max_bytes=512,
                ),
                "evidence": _require_text(
                    blocker.get("evidence"),
                    f"architecture review blockers[{index}].evidence",
                    max_bytes=512,
                ),
                "affected_files": _string_list(
                    blocker.get("affected_files"),
                    f"architecture review blockers[{index}].affected_files",
                    max_items=8,
                    item_bytes=256,
                ),
                "regression_check": _safe_id(
                    blocker.get("regression_check"),
                    f"architecture review blockers[{index}].regression_check",
                ),
            }
        )
    planned_gaps = _string_list(
        review.get("planned_gaps"),
        "architecture review planned_gaps",
        max_items=8,
        item_bytes=MAX_REVIEW_INPUT_BYTES,
    )
    recommendations = _string_list(
        review.get("recommendations"),
        "architecture review recommendations",
        max_items=8,
        item_bytes=MAX_REVIEW_INPUT_BYTES,
    )
    checks = _check_list(review.get("checks"), "architecture review checks")
    if verdict == "passed" and normalized_blockers:
        raise StateCtlError("passed architecture review must not contain blockers")
    if verdict == "passed" and any(check["status"] == "failed" for check in checks):
        raise StateCtlError("passed architecture review must not contain failed checks")
    if verdict == "blocked" and not normalized_blockers:
        raise StateCtlError("blocked architecture review requires at least one blocker")
    return {
        "protocol": ARCHITECTURE_REVIEW_PROTOCOL,
        "verdict": verdict,
        "summary": summary,
        "blockers": normalized_blockers,
        "planned_gaps": planned_gaps,
        "recommendations": recommendations,
        "checks": checks,
    }


def _compact_architecture_review(review: dict[str, Any]) -> str:
    """Keep typed review classes in state; full review remains in the run artifact."""
    compact = {
        "verdict": review["verdict"],
        "summary": _truncate_utf8(review["summary"], 256),
        "blockers": {
            "count": len(review["blockers"]),
            "sample": [
                _truncate_utf8(item["contract"], 96) for item in review["blockers"][:2]
            ],
        },
        "planned_gaps": {
            "count": len(review["planned_gaps"]),
            "sample": [_truncate_utf8(item, 96) for item in review["planned_gaps"][:2]],
        },
        "recommendations": {
            "count": len(review["recommendations"]),
            "sample": [_truncate_utf8(item, 96) for item in review["recommendations"][:2]],
        },
        "checks": [
            {"id": item["id"], "status": item["status"]} for item in review["checks"][:8]
        ],
    }
    encoded = json.dumps(compact, ensure_ascii=False, separators=(",", ":"))
    if _byte_len(encoded) > MAX_REVIEW_RECORD_BYTES:
        compact["summary"] = _truncate_utf8(review["summary"], 96)
        compact["checks"] = compact["checks"][:4]
        for field in ("blockers", "planned_gaps", "recommendations"):
            compact[field]["sample"] = compact[field]["sample"][:1]
        encoded = json.dumps(compact, ensure_ascii=False, separators=(",", ":"))
    if _byte_len(encoded) > MAX_REVIEW_RECORD_BYTES:
        raise StateCtlError("compacted architecture review exceeds state limit")
    return encoded


def _read_json(path: Path, label: str) -> dict[str, Any]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError as exc:
        raise StateCtlError(f"{label} not found: {path}") from exc
    except json.JSONDecodeError as exc:
        raise StateCtlError(f"invalid JSON in {label} {path}: {exc}") from exc
    if not isinstance(value, dict):
        raise StateCtlError(f"{label} must contain a JSON object: {path}")
    return value


def _atomic_write_bytes(path: Path, payload: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    previous_mode: int | None = None
    try:
        previous_mode = path.stat().st_mode & 0o777
    except FileNotFoundError:
        pass
    temporary: str | None = None
    try:
        with tempfile.NamedTemporaryFile(dir=path.parent, delete=False) as handle:
            temporary = handle.name
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        if previous_mode is not None:
            os.chmod(temporary, previous_mode)
        os.replace(temporary, path)
        _fsync_directory(path.parent)
    finally:
        if temporary and os.path.exists(temporary):
            os.unlink(temporary)


def _atomic_write_json(path: Path, value: dict[str, Any]) -> None:
    _atomic_write_bytes(path, _json_bytes(value))


def _fsync_directory(path: Path) -> None:
    """Persist directory-entry changes where the host filesystem supports it."""

    flags = os.O_RDONLY | getattr(os, "O_DIRECTORY", 0)
    try:
        descriptor = os.open(path, flags)
    except OSError:
        return
    try:
        os.fsync(descriptor)
    except OSError:
        pass
    finally:
        os.close(descriptor)


def _atomic_create_bytes(path: Path, payload: bytes) -> None:
    """Publish a complete new file without ever replacing an existing path."""

    path.parent.mkdir(parents=True, exist_ok=True)
    temporary: str | None = None
    try:
        with tempfile.NamedTemporaryFile(dir=path.parent, delete=False) as handle:
            temporary = handle.name
            handle.write(payload)
            handle.flush()
            os.fsync(handle.fileno())
        try:
            os.link(temporary, path)
        except FileExistsError as exc:
            raise StateCtlError(
                "packet output already exists; choose a new path"
            ) from exc
        _fsync_directory(path.parent)
    finally:
        if temporary and os.path.exists(temporary):
            os.unlink(temporary)


def _sha256(payload: bytes) -> str:
    return hashlib.sha256(payload).hexdigest()


def _transaction_paths(state_path: Path) -> tuple[Path, Path]:
    return (
        state_path.parent / TRANSACTION_MARKER,
        state_path.parent / TRANSACTION_BACKUP,
    )


def _remove_transaction_files(state_path: Path) -> None:
    marker, backup = _transaction_paths(state_path)
    try:
        marker.unlink()
        _fsync_directory(marker.parent)
    except FileNotFoundError:
        pass
    try:
        backup.unlink()
        _fsync_directory(backup.parent)
    except FileNotFoundError:
        pass


def _authority_path_from_state(state_path: Path, root: Path) -> tuple[Path, str]:
    state = _read_json(state_path, "state during transaction recovery")
    source = state.get("source")
    if not isinstance(source, dict):
        raise StateCtlError("cannot recover transaction with an invalid state source")
    if source.get("kind") == "standalone":
        return (
            _inside(_task_ledger_path(state_path), root, "standalone task ledger"),
            "standalone",
        )
    if source.get("kind") == "openspec":
        tasks_path = _require_text(source.get("tasks_path"), "source.tasks_path")
        return _inside(root / tasks_path, root, "OpenSpec tasks path"), "openspec"
    raise StateCtlError("cannot recover transaction with an unknown state source")


def _recover_pair_transaction(
    root: Path, state_path: Path, *, authority_locked: bool = False
) -> None:
    """Finish or roll back an interrupted authority/state pair write."""

    marker, backup = _transaction_paths(state_path)
    if not marker.exists():
        # A crash before publishing the marker can leave only the inert backup.
        if backup.exists():
            backup.unlink()
            _fsync_directory(backup.parent)
        return
    if marker.stat().st_size > 4096:
        raise StateCtlError(
            "transaction marker is too large; manual recovery is required",
            kind="transaction_recovery",
            code=ERROR_CONFLICT,
        )
    transaction = _read_json(marker, "transaction marker")
    expected_fields = {
        "version",
        "authority_path",
        "authority_old_sha256",
        "authority_new_sha256",
        "state_old_sha256",
        "state_new_sha256",
    }
    if set(transaction) != expected_fields or transaction.get("version") != TRANSACTION_VERSION:
        raise StateCtlError(
            "invalid transaction marker; manual recovery is required",
            kind="transaction_recovery",
            code=ERROR_CONFLICT,
        )
    authority_relative = _require_text(
        transaction.get("authority_path"), "transaction authority_path"
    )
    authority_path = _inside(root / authority_relative, root, "transaction authority path")
    expected_authority, source_kind = _authority_path_from_state(state_path, root)
    if authority_path != expected_authority:
        raise StateCtlError(
            "transaction authority does not match state; manual recovery is required",
            kind="transaction_recovery",
            code=ERROR_CONFLICT,
        )
    if source_kind == "openspec" and not authority_locked:
        with _authority_lock(authority_path):
            _recover_pair_transaction(root, state_path, authority_locked=True)
        return
    try:
        old_authority = backup.read_bytes()
        authority = authority_path.read_bytes()
        state = state_path.read_bytes()
    except FileNotFoundError as exc:
        raise StateCtlError(
            "transaction recovery file is missing; manual recovery is required",
            kind="transaction_recovery",
            code=ERROR_CONFLICT,
        ) from exc
    hashes = {
        key: transaction[key]
        for key in expected_fields
        if key.endswith("sha256")
    }
    if not all(
        isinstance(value, str) and re.fullmatch(r"[0-9a-f]{64}", value)
        for value in hashes.values()
    ):
        raise StateCtlError(
            "invalid transaction hashes; manual recovery is required",
            kind="transaction_recovery",
            code=ERROR_CONFLICT,
        )
    if _sha256(old_authority) != transaction["authority_old_sha256"]:
        raise StateCtlError(
            "transaction backup is invalid; manual recovery is required",
            kind="transaction_recovery",
            code=ERROR_CONFLICT,
        )
    authority_hash = _sha256(authority)
    state_hash = _sha256(state)
    old_pair = (
        transaction["authority_old_sha256"],
        transaction["state_old_sha256"],
    )
    new_pair = (
        transaction["authority_new_sha256"],
        transaction["state_new_sha256"],
    )
    current_pair = (authority_hash, state_hash)
    if current_pair == new_pair or current_pair == old_pair:
        _remove_transaction_files(state_path)
        return
    if current_pair == (new_pair[0], old_pair[1]):
        # Authority was replaced, but state was not. Restore only if its bytes
        # are exactly the bytes written by this transaction.
        _atomic_write_bytes(authority_path, old_authority)
        _remove_transaction_files(state_path)
        return
    raise StateCtlError(
        "transaction files changed concurrently; manual recovery is required",
        kind="transaction_recovery",
        code=ERROR_CONFLICT,
    )


def _write_pair_with_rollback(
    root: Path,
    first_path: Path,
    first_payload: bytes,
    second_path: Path,
    second_payload: bytes,
    *,
    expected_first: bytes | None = None,
) -> None:
    """Journaled two-file update; an interrupted write is recovered on next use."""

    old_first = first_path.read_bytes()
    old_second = second_path.read_bytes()
    if expected_first is not None and old_first != expected_first:
        raise StateCtlError(
            f"concurrent authority-file change detected: {first_path}",
            kind="revision_conflict",
            code=ERROR_CONFLICT,
        )
    marker, backup = _transaction_paths(second_path)
    if marker.exists():
        raise StateCtlError(
            "unfinished transaction must be recovered before a new write",
            kind="transaction_recovery",
            code=ERROR_CONFLICT,
        )
    _atomic_write_bytes(backup, old_first)
    transaction = {
        "version": TRANSACTION_VERSION,
        "authority_path": _inside(first_path, root, "authority path")
        .relative_to(root)
        .as_posix(),
        "authority_old_sha256": _sha256(old_first),
        "authority_new_sha256": _sha256(first_payload),
        "state_old_sha256": _sha256(old_second),
        "state_new_sha256": _sha256(second_payload),
    }
    _atomic_write_json(marker, transaction)
    if first_path.read_bytes() != old_first or second_path.read_bytes() != old_second:
        _remove_transaction_files(second_path)
        raise StateCtlError(
            "concurrent file change detected before transaction commit",
            kind="revision_conflict",
            code=ERROR_CONFLICT,
        )
    try:
        _atomic_write_bytes(first_path, first_payload)
        _atomic_write_bytes(second_path, second_payload)
    except Exception:
        _recover_pair_transaction(root, second_path)
        raise
    _remove_transaction_files(second_path)


def _root(value: str) -> Path:
    root = Path(value).expanduser().resolve()
    if not root.is_dir():
        raise StateCtlError(f"project root not found: {root}")
    return root


def _inside(path: Path, root: Path, field: str) -> Path:
    resolved = path.expanduser().resolve()
    try:
        resolved.relative_to(root)
    except ValueError as exc:
        raise StateCtlError(f"{field} must be inside project root: {resolved}") from exc
    return resolved


def _state_path(args: argparse.Namespace, *, must_exist: bool = True) -> tuple[Path, Path]:
    root = _root(args.project_root)
    explicit = getattr(args, "state", None)
    identifier = getattr(args, "id", None)
    if explicit:
        candidate = Path(explicit)
        path = candidate if candidate.is_absolute() else root / candidate
    elif identifier:
        path = root / ".execution-state" / _safe_id(identifier) / "state.json"
    else:
        raise StateCtlError("provide --state or --id")
    path = _inside(path, root, "state path")
    if must_exist and not path.is_file():
        raise StateCtlError(f"state not found: {path}")
    return root, path


def _pid_is_alive(pid: int) -> bool:
    if pid <= 0:
        return False
    try:
        os.kill(pid, 0)
    except ProcessLookupError:
        return False
    except PermissionError:
        return True
    except OSError as exc:
        return exc.errno != errno.ESRCH
    return True


def _reclaim_stale_lock_directory(lock_path: Path) -> bool:
    """Migrate a stale lock directory left by the older lock implementation."""

    try:
        stat = lock_path.stat()
    except FileNotFoundError:
        return True
    if not lock_path.is_dir():
        return False
    stale = time.time() - stat.st_mtime >= LOCK_DIRECTORY_STALE_SECONDS
    owner_path = lock_path / "owner.json"
    try:
        owner = _read_json(owner_path, "lock owner")
    except StateCtlError:
        owner = None
    if isinstance(owner, dict):
        pid = owner.get("pid")
        if isinstance(pid, int) and not isinstance(pid, bool):
            stale = not _pid_is_alive(pid)
    if not stale:
        return False
    reaped = lock_path.with_name(f"{lock_path.name}.reaped-{uuid.uuid4().hex}")
    try:
        lock_path.rename(reaped)
    except FileNotFoundError:
        return True
    except OSError:
        return False
    shutil.rmtree(reaped, ignore_errors=True)
    return True


@contextmanager
def _portable_lock(lock_path: Path, *, label: str, kind: str):
    """Use a crash-safe OS lock on POSIX and an owned directory elsewhere."""

    lock_path.parent.mkdir(parents=True, exist_ok=True)
    busy = lambda: StateCtlError(label, kind=kind, code=ERROR_CONFLICT)
    if fcntl is not None:
        if lock_path.is_dir() and not _reclaim_stale_lock_directory(lock_path):
            raise busy()
        try:
            descriptor = os.open(lock_path, os.O_RDWR | os.O_CREAT, 0o600)
        except (FileExistsError, IsADirectoryError) as exc:
            raise busy() from exc
        acquired = False
        try:
            try:
                fcntl.flock(descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
                acquired = True
            except (BlockingIOError, PermissionError) as exc:
                raise busy() from exc
            owner = _json_bytes({"pid": os.getpid(), "created_ns": time.time_ns()})
            os.ftruncate(descriptor, 0)
            os.write(descriptor, owner)
            os.fsync(descriptor)
            yield
        finally:
            if acquired:
                try:
                    held = os.fstat(descriptor)
                    current = lock_path.stat()
                    if (held.st_dev, held.st_ino) == (current.st_dev, current.st_ino):
                        lock_path.unlink()
                        _fsync_directory(lock_path.parent)
                except FileNotFoundError:
                    pass
                finally:
                    fcntl.flock(descriptor, fcntl.LOCK_UN)
            os.close(descriptor)
        return

    # A portable fallback retains PID ownership and reclaims a dead/old owner.
    token = uuid.uuid4().hex
    for attempt in range(2):
        try:
            lock_path.mkdir()
            break
        except FileExistsError as exc:
            if attempt == 0 and _reclaim_stale_lock_directory(lock_path):
                continue
            raise busy() from exc
    owner_path = lock_path / "owner.json"
    try:
        _atomic_write_json(
            owner_path,
            {"pid": os.getpid(), "token": token, "created_ns": time.time_ns()},
        )
        yield
    finally:
        try:
            owner = _read_json(owner_path, "lock owner")
            if owner.get("token") == token:
                owner_path.unlink()
                lock_path.rmdir()
        except (OSError, StateCtlError):
            pass


@contextmanager
def _state_lock(state_path: Path):
    """Serialize all access to one state and recover interrupted pair writes."""

    lock_path = state_path.parent / ".statectl.lock"
    with _portable_lock(
        lock_path,
        label=f"state is busy: {state_path}",
        kind="state_busy",
    ):
        yield


@contextmanager
def _authority_lock(authority_path: Path):
    """Serialize cooperating writers of one OpenSpec authority file."""

    lock_path = _authority_lock_path(authority_path)
    with _portable_lock(
        lock_path,
        label=f"authority file is busy: {authority_path}",
        kind="authority_busy",
    ):
        yield


def _authority_lock_path(authority_path: Path) -> Path:
    return authority_path.parent / f".{authority_path.name}.statectl.lock"


def _locked_state_command(handler):
    def invoke(args: argparse.Namespace) -> dict[str, Any]:
        root, path = _state_path(args)
        with _state_lock(path):
            _recover_pair_transaction(root, path)
            return handler(args)

    return invoke


def _locked_init_command(handler):
    def invoke(args: argparse.Namespace) -> dict[str, Any]:
        root, path = _state_path(args, must_exist=False)
        path.parent.mkdir(parents=True, exist_ok=True)
        try:
            with _state_lock(path):
                return handler(args)
        finally:
            if not path.exists():
                for directory in (path.parent, root / ".execution-state"):
                    try:
                        directory.rmdir()
                    except OSError:
                        pass

    return invoke


def _relative_project_path(path: Path, root: Path) -> str:
    return path.relative_to(root).as_posix()


def _contains_banned_key(value: Any) -> str | None:
    if isinstance(value, dict):
        for key, nested in value.items():
            if key in _BANNED_STATE_KEYS:
                return key
            found = _contains_banned_key(nested)
            if found:
                return found
    elif isinstance(value, list):
        for nested in value:
            found = _contains_banned_key(nested)
            if found:
                return found
    return None


def _parse_openspec_tasks(tasks_path: Path) -> tuple[str, list[dict[str, Any]]]:
    try:
        text = tasks_path.read_text(encoding="utf-8")
    except FileNotFoundError as exc:
        raise StateCtlError(f"OpenSpec tasks file not found: {tasks_path}") from exc
    tasks: list[dict[str, Any]] = []
    seen: set[str] = set()
    for line_number, line_with_ending in enumerate(text.splitlines(keepends=True), start=1):
        line = line_with_ending.rstrip("\r\n")
        match = _TASK_LINE.match(line)
        if not match:
            continue
        task_id = match.group("id")
        if task_id in seen:
            raise StateCtlError(
                f"duplicate OpenSpec task id {task_id!r} at line {line_number}"
            )
        seen.add(task_id)
        tasks.append(
            {
                "id": task_id,
                "title": match.group("title").strip(),
                "complete": match.group("mark").lower() == "x",
                "line": line_number,
            }
        )
    if not tasks:
        raise StateCtlError(
            "OpenSpec tasks.md has no executable checkbox tasks with stable ids"
        )
    return text, tasks


def _check_openspec_task(tasks: list[dict[str, Any]], task_id: str) -> dict[str, Any]:
    matches = [task for task in tasks if task["id"] == task_id]
    if not matches:
        raise StateCtlError(f"OpenSpec task not found: {task_id}")
    return matches[0]


def _mark_openspec_complete(text: str, line_number: int) -> str:
    lines = text.splitlines(keepends=True)
    line = lines[line_number - 1]
    ending = "\r\n" if line.endswith("\r\n") else "\n" if line.endswith("\n") else ""
    body = line[: -len(ending)] if ending else line
    match = _TASK_LINE.match(body)
    if not match:
        raise StateCtlError("OpenSpec task line changed before completion")
    start, end = match.span("mark")
    lines[line_number - 1] = body[:start] + "x" + body[end:] + ending
    return "".join(lines)


def _task_ledger_path(state_path: Path) -> Path:
    return state_path.parent / "tasks.json"


def _context_map_path(state_path: Path) -> Path:
    return state_path.parent / "context-map.json"


def _normalize_task(raw: Any, index: int) -> dict[str, Any]:
    if not isinstance(raw, dict):
        raise StateCtlError(f"tasks[{index}] must be an object")
    task_id = _safe_id(raw.get("id"), f"tasks[{index}].id")
    title = _require_text(raw.get("title"), f"tasks[{index}].title", max_bytes=1024)
    done_when = _string_list(
        raw.get("done_when"),
        f"tasks[{index}].done_when",
        max_items=8,
        item_bytes=1024,
    )
    if not done_when:
        raise StateCtlError(f"tasks[{index}].done_when must not be empty")
    kind = raw.get("kind", "implementation")
    if kind not in TASK_KINDS:
        raise StateCtlError(f"tasks[{index}].kind is invalid")
    affected_areas = _string_list(raw.get("affected_areas", []), f"tasks[{index}].affected_areas", max_items=8, item_bytes=128)
    cohesion_key = _safe_id(
        raw.get("cohesion_key") or (affected_areas[0] if affected_areas else task_id),
        f"tasks[{index}].cohesion_key",
    )
    requires_bridge = raw.get("requires_bridge", False)
    if not isinstance(requires_bridge, bool):
        raise StateCtlError(f"tasks[{index}].requires_bridge must be boolean")
    return {
        "id": task_id,
        "title": title,
        "status": "pending",
        "done_when": done_when,
        "kind": kind,
        "cohesion_key": cohesion_key,
        "affected_areas": affected_areas,
        "reads": _string_list(raw.get("reads", []), f"tasks[{index}].reads", max_items=16, item_bytes=256),
        "writes": _string_list(raw.get("writes", []), f"tasks[{index}].writes", max_items=16, item_bytes=256),
        "contracts": _string_list(raw.get("contracts", []), f"tasks[{index}].contracts", max_items=8, item_bytes=512),
        "regression_checks": _string_list(raw.get("regression_checks", []), f"tasks[{index}].regression_checks", max_items=8, item_bytes=128),
        "requires_bridge": requires_bridge,
    }


def _validate_task_ledger(ledger: dict[str, Any]) -> list[dict[str, Any]]:
    if ledger.get("schema_version") != TASK_LEDGER_VERSION:
        raise StateCtlError(f"task ledger schema_version must equal {TASK_LEDGER_VERSION}")
    if set(ledger) != {"schema_version", "tasks"}:
        raise StateCtlError("task ledger has unsupported fields")
    tasks = ledger.get("tasks")
    if not isinstance(tasks, list) or not tasks:
        raise StateCtlError("task ledger must contain at least one task")
    seen: set[str] = set()
    for index, task in enumerate(tasks):
        if not isinstance(task, dict):
            raise StateCtlError(f"tasks[{index}] must be an object")
        allowed = {
            "id", "title", "status", "done_when", "kind", "cohesion_key",
            "affected_areas", "reads", "writes", "contracts", "regression_checks",
            "requires_bridge", "evidence", "checks", "summary",
        }
        if set(task) - allowed:
            raise StateCtlError(f"tasks[{index}] has unsupported fields")
        task_id = _safe_id(task.get("id"), f"tasks[{index}].id")
        if task_id in seen:
            raise StateCtlError(f"duplicate standalone task id: {task_id}")
        seen.add(task_id)
        _require_text(task.get("title"), f"tasks[{index}].title", max_bytes=1024)
        done_when = _string_list(
            task.get("done_when"),
            f"tasks[{index}].done_when",
            max_items=8,
            item_bytes=1024,
        )
        if not done_when:
            raise StateCtlError(f"tasks[{index}].done_when must not be empty")
        if task.get("status") not in TASK_STATUSES:
            raise StateCtlError(f"tasks[{index}].status is invalid")
        if task.get("kind") not in TASK_KINDS:
            raise StateCtlError(f"tasks[{index}].kind is invalid")
        _safe_id(task.get("cohesion_key"), f"tasks[{index}].cohesion_key")
        _string_list(task.get("affected_areas"), f"tasks[{index}].affected_areas", max_items=8, item_bytes=128)
        _string_list(task.get("reads"), f"tasks[{index}].reads", max_items=16, item_bytes=256)
        _string_list(task.get("writes"), f"tasks[{index}].writes", max_items=16, item_bytes=256)
        _string_list(task.get("contracts"), f"tasks[{index}].contracts", max_items=8, item_bytes=512)
        regression_checks = _string_list(task.get("regression_checks"), f"tasks[{index}].regression_checks", max_items=8, item_bytes=128)
        if not isinstance(task.get("requires_bridge"), bool):
            raise StateCtlError(f"tasks[{index}].requires_bridge must be boolean")
        if task.get("status") == "complete":
            evidence = _evidence_list(task.get("evidence"), f"tasks[{index}].evidence")
            checks = _check_list(task.get("checks", []), f"tasks[{index}].checks")
            if not evidence and not checks:
                raise StateCtlError(f"tasks[{index}] complete without evidence")
            passed = {check["id"] for check in checks if check["status"] == "passed"}
            missing = [check_id for check_id in regression_checks if check_id not in passed]
            if missing:
                raise StateCtlError(f"tasks[{index}] missing passed checks: {', '.join(missing)}")
            _require_text(task.get("summary"), f"tasks[{index}].summary", max_bytes=1024)
    if len(_json_bytes(ledger)) > MAX_TASK_LEDGER_BYTES:
        raise StateCtlError(f"task ledger exceeds {MAX_TASK_LEDGER_BYTES} bytes")
    return tasks


def _load_ledger(state_path: Path) -> tuple[Path, dict[str, Any], list[dict[str, Any]]]:
    path = _task_ledger_path(state_path)
    ledger = _read_json(path, "standalone task ledger")
    return path, ledger, _validate_task_ledger(ledger)


def _validate_capabilities(value: Any, field: str) -> dict[str, bool]:
    if not isinstance(value, dict) or not value:
        raise StateCtlError(f"{field} must be a non-empty capability object")
    if not all(isinstance(key, str) and isinstance(enabled, bool) for key, enabled in value.items()):
        raise StateCtlError(f"{field} must map strings to booleans")
    return value


def _validate_state(
    state: dict[str, Any],
    state_path: Path,
    root: Path,
    *,
    check_size: bool = True,
    ledger_override: dict[str, Any] | None = None,
    openspec_text_override: str | None = None,
) -> None:
    allowed = {
        "schema_version",
        "id",
        "revision",
        "source",
        "execution",
        "implementation_ref",
        "goal",
        "status",
        "constraints",
        "active_task",
        "next_action",
        "observation",
        "blocker",
        "artifacts",
        "checkpoint",
        "last_result",
        "worker_lease",
        "quality",
    }
    extra = sorted(set(state) - allowed)
    if extra:
        raise StateCtlError(f"state has unsupported fields: {', '.join(extra)}")
    banned = _contains_banned_key(state)
    if banned:
        raise StateCtlError(f"state must not contain {banned!r}")
    if state.get("schema_version") != SCHEMA_VERSION:
        raise StateCtlError(f"schema_version must equal {SCHEMA_VERSION}")
    _safe_id(state.get("id"))
    revision = state.get("revision")
    if not isinstance(revision, int) or isinstance(revision, bool) or revision < 0:
        raise StateCtlError("revision must be a non-negative integer")
    _require_text(state.get("implementation_ref"), "implementation_ref", max_bytes=1024)
    _require_text(state.get("goal"), "goal")
    _string_list(state.get("constraints"), "constraints", max_items=32, item_bytes=1024)
    if state.get("status") not in STATE_STATUSES:
        raise StateCtlError("state.status is invalid")

    source = state.get("source")
    if not isinstance(source, dict) or source.get("kind") not in SOURCE_KINDS:
        raise StateCtlError("source.kind must be standalone or openspec")
    if source["kind"] == "standalone":
        if set(source) != {"kind", "tasks_ref"} or source.get("tasks_ref") != "tasks.json":
            raise StateCtlError("standalone source must reference tasks.json")
        if ledger_override is None:
            _, _, source_tasks = _load_ledger(state_path)
        else:
            source_tasks = _validate_task_ledger(ledger_override)
    else:
        if set(source) != {"kind", "change", "tasks_path"}:
            raise StateCtlError("openspec source fields are invalid")
        _safe_id(source.get("change"), "source.change")
        tasks_path = _inside(root / _require_text(source.get("tasks_path"), "source.tasks_path"), root, "OpenSpec tasks path")
        if openspec_text_override is None:
            _, openspec_tasks = _parse_openspec_tasks(tasks_path)
        else:
            _, openspec_tasks = _parse_openspec_tasks_from_text(openspec_text_override)
        if state.get("status") == "complete" and any(
            not task["complete"] for task in openspec_tasks
        ):
            raise StateCtlError("complete state has unchecked OpenSpec tasks")

    execution = state.get("execution")
    if not isinstance(execution, dict) or set(execution) != {
        "profile",
        "adapter",
        "capabilities",
    }:
        raise StateCtlError("execution fields are invalid")
    if execution.get("profile") not in EXECUTION_PROFILES:
        raise StateCtlError("execution.profile must be lite or reset")
    _require_text(execution.get("adapter"), "execution.adapter", max_bytes=64)
    capabilities = _validate_capabilities(execution.get("capabilities"), "execution.capabilities")
    if execution["profile"] == "reset" and not any(
        capabilities.get(name) is True
        for name in ("fresh_context", "in_place_compaction")
    ):
        raise StateCtlError("reset profile requires a confirmed reset capability")

    active = state.get("active_task")
    if active is not None:
        if not isinstance(active, dict):
            raise StateCtlError("active_task must be null or an object")
        expected_active = {
            "id",
            "title",
            "status",
            "done_when",
            "evidence",
            "checks",
            "artifacts",
            "kind",
            "cohesion_key",
            "affected_areas",
            "reads",
            "writes",
            "contracts",
            "regression_checks",
            "requires_bridge",
        }
        if set(active) != expected_active:
            raise StateCtlError("active_task fields are invalid")
        _safe_id(active.get("id"), "active_task.id")
        _require_text(active.get("title"), "active_task.title", max_bytes=1024)
        if active.get("status") not in {"in_progress", "blocked"}:
            raise StateCtlError("active_task.status must be in_progress or blocked")
        _string_list(
            active.get("done_when"),
            "active_task.done_when",
            max_items=8,
            item_bytes=1024,
        )
        _evidence_list(active.get("evidence"), "active_task.evidence")
        _check_list(active.get("checks"), "active_task.checks")
        _string_list(active.get("artifacts"), "active_task.artifacts", item_bytes=1024)
        if active.get("kind") not in TASK_KINDS:
            raise StateCtlError("active_task.kind is invalid")
        _safe_id(active.get("cohesion_key"), "active_task.cohesion_key")
        _string_list(active.get("affected_areas"), "active_task.affected_areas", max_items=8, item_bytes=128)
        _string_list(active.get("reads"), "active_task.reads", max_items=16, item_bytes=256)
        _string_list(active.get("writes"), "active_task.writes", max_items=16, item_bytes=256)
        _string_list(active.get("contracts"), "active_task.contracts", max_items=8, item_bytes=512)
        _string_list(active.get("regression_checks"), "active_task.regression_checks", max_items=8, item_bytes=128)
        if not isinstance(active.get("requires_bridge"), bool):
            raise StateCtlError("active_task.requires_bridge must be boolean")
    if state["status"] in {"in_progress", "blocked"} and active is None:
        raise StateCtlError(f"{state['status']} state requires an active task")
    if state["status"] == "complete" and active is not None:
        raise StateCtlError("complete state must not have an active task")

    if source["kind"] == "standalone":
        active_source = (
            next((task for task in source_tasks if active and task["id"] == active["id"]), None)
            if active
            else None
        )
        if active is not None and active_source is None:
            raise StateCtlError("active task is absent from standalone task ledger")
        if active_source is not None and active_source["status"] != active["status"]:
            raise StateCtlError("active task status disagrees with standalone task ledger")
        if active_source is not None and (
            active_source["title"] != active["title"]
            or active_source["done_when"] != active["done_when"]
            or any(active_source[field] != active[field] for field in (
                "kind", "cohesion_key", "affected_areas", "reads", "writes",
                "contracts", "regression_checks", "requires_bridge",
            ))
        ):
            raise StateCtlError("active task definition drifted from standalone task ledger")
        unfinished = [task for task in source_tasks if task["status"] != "complete"]
        if state["status"] == "complete" and unfinished:
            raise StateCtlError("complete state has unfinished standalone tasks")
        if state["status"] != "complete" and not unfinished:
            raise StateCtlError("all standalone tasks are complete but state is not complete")
    else:
        active_source = (
            next((task for task in openspec_tasks if active and task["id"] == active["id"]), None)
            if active
            else None
        )
        if active is not None and active_source is None:
            raise StateCtlError("active task is absent from OpenSpec tasks.md")
        if active_source is not None and active_source["complete"]:
            raise StateCtlError("active OpenSpec task is already checked")
        if active_source is not None and (
            active_source["title"] != active["title"]
            or active["done_when"] != [active_source["title"]]
        ):
            raise StateCtlError("active task definition drifted from OpenSpec tasks.md")
        if state["status"] != "complete" and all(task["complete"] for task in openspec_tasks):
            raise StateCtlError("all OpenSpec tasks are checked but state is not complete")

    next_action = state.get("next_action")
    if state["status"] != "complete":
        _require_text(next_action, "next_action", max_bytes=1024)
    elif next_action not in {None, ""}:
        raise StateCtlError("complete state next_action must be null")
    observation = state.get("observation")
    if not isinstance(observation, str) or _byte_len(observation) > MAX_OBSERVATION_BYTES:
        raise StateCtlError(f"observation must be a string up to {MAX_OBSERVATION_BYTES} bytes")
    blocker = state.get("blocker")
    if blocker is not None:
        _require_text(blocker, "blocker", max_bytes=MAX_OBSERVATION_BYTES)
    if state["status"] == "blocked" and blocker is None:
        raise StateCtlError("blocked state requires blocker")
    _string_list(state.get("artifacts"), "artifacts", item_bytes=1024)

    checkpoint = state.get("checkpoint")
    if not isinstance(checkpoint, dict) or set(checkpoint) != {"ready", "reason", "revision"}:
        raise StateCtlError("checkpoint fields are invalid")
    if not isinstance(checkpoint.get("ready"), bool):
        raise StateCtlError("checkpoint.ready must be boolean")
    _require_text(checkpoint.get("reason"), "checkpoint.reason", max_bytes=1024)
    if checkpoint.get("revision") != revision:
        raise StateCtlError("checkpoint.revision must equal state revision")

    last_result = state.get("last_result")
    if last_result is not None:
        if not isinstance(last_result, dict) or set(last_result) != {
            "id",
            "summary",
            "evidence",
            "checks",
        }:
            raise StateCtlError("last_result fields are invalid")
        _safe_id(last_result.get("id"), "last_result.id")
        _require_text(last_result.get("summary"), "last_result.summary", max_bytes=1024)
        if not _evidence_list(last_result.get("evidence"), "last_result.evidence"):
            if not _check_list(last_result.get("checks"), "last_result.checks"):
                raise StateCtlError("last_result requires evidence or checks")
        else:
            _check_list(last_result.get("checks"), "last_result.checks")

    quality = state.get("quality")
    if not isinstance(quality, dict) or set(quality) != {
        "invariants", "completed_chunks", "review_interval", "review_required",
        "review_reasons", "last_review", "pending_bridge", "next_handoff",
    }:
        raise StateCtlError("quality fields are invalid")
    _string_list(quality.get("invariants"), "quality.invariants", max_items=16, item_bytes=512)
    completed_chunks = quality.get("completed_chunks")
    if not isinstance(completed_chunks, int) or isinstance(completed_chunks, bool) or completed_chunks < 0:
        raise StateCtlError("quality.completed_chunks must be a non-negative integer")
    review_interval = quality.get("review_interval")
    if not isinstance(review_interval, int) or isinstance(review_interval, bool) or review_interval < 2 or review_interval > 32:
        raise StateCtlError("quality.review_interval must be between 2 and 32")
    if not isinstance(quality.get("review_required"), bool) or not isinstance(quality.get("pending_bridge"), bool):
        raise StateCtlError("quality review/bridge flags must be boolean")
    _string_list(quality.get("review_reasons"), "quality.review_reasons", max_items=4, item_bytes=256)
    if quality.get("last_review") is not None:
        _require_text(quality.get("last_review"), "quality.last_review", max_bytes=1024)
    if quality.get("next_handoff") not in {"continue", "reset", "checkpoint"}:
        raise StateCtlError("quality.next_handoff is invalid")

    worker_lease = state.get("worker_lease")
    if worker_lease is not None:
        if not isinstance(worker_lease, dict) or set(worker_lease) != {
            "run_id",
            "task_id",
            "based_on_revision",
        }:
            raise StateCtlError("worker_lease fields are invalid")
        _safe_id(worker_lease.get("run_id"), "worker_lease.run_id")
        _safe_id(worker_lease.get("task_id"), "worker_lease.task_id")
        if worker_lease.get("based_on_revision") != revision:
            raise StateCtlError("worker_lease revision must equal state revision")
        if active is None or worker_lease["task_id"] != active["id"]:
            raise StateCtlError("worker_lease must reference the active task")
        if active["status"] != "in_progress" or not checkpoint["ready"]:
            raise StateCtlError("worker_lease requires a ready in-progress task")

    if check_size and len(_json_bytes(state)) > MAX_STATE_BYTES:
        raise StateCtlError(f"state exceeds {MAX_STATE_BYTES} bytes")


def _load_state(args: argparse.Namespace) -> tuple[Path, Path, dict[str, Any]]:
    root, path = _state_path(args)
    if path.stat().st_size > MAX_STATE_BYTES:
        raise StateCtlError(f"state exceeds {MAX_STATE_BYTES} bytes")
    state = _read_json(path, "state")
    _validate_state(state, path, root)
    return root, path, state


def _expect_revision(state: dict[str, Any], expected: int) -> None:
    if state["revision"] != expected:
        raise StateCtlError(
            f"revision conflict: expected {expected}, current {state['revision']}",
            kind="revision_conflict",
            code=ERROR_CONFLICT,
        )


def _advance(state: dict[str, Any], *, checkpoint_ready: bool, reason: str) -> None:
    state["revision"] += 1
    state["checkpoint"] = {
        "ready": checkpoint_ready,
        "reason": _require_text(reason, "checkpoint reason", max_bytes=1024),
        "revision": state["revision"],
    }


def _release_worker_lease(state: dict[str, Any], supplied_run_id: str | None) -> None:
    lease = state.get("worker_lease")
    if lease is None:
        return
    if supplied_run_id != lease["run_id"]:
        raise StateCtlError(
            "active worker lease requires the matching --run-id",
            kind="worker_lease_conflict",
            code=ERROR_CONFLICT,
        )
    state["worker_lease"] = None


def _next_pending(tasks: Iterable[dict[str, Any]]) -> dict[str, Any] | None:
    return next((task for task in tasks if task.get("status") != "complete" and not task.get("complete", False)), None)


def _active_from_task(task: dict[str, Any], source_kind: str) -> dict[str, Any]:
    done_when = task.get("done_when", []) if source_kind == "standalone" else [task["title"]]
    return {
        "id": task["id"],
        "title": task["title"],
        "status": "in_progress",
        "done_when": list(done_when),
        "evidence": [],
        "checks": [],
        "artifacts": [],
        "kind": task.get("kind", "implementation"),
        "cohesion_key": task.get("cohesion_key", task["id"]),
        "affected_areas": list(task.get("affected_areas", [])),
        "reads": list(task.get("reads", [])),
        "writes": list(task.get("writes", [])),
        "contracts": list(task.get("contracts", [])),
        "regression_checks": list(task.get("regression_checks", [])),
        "requires_bridge": bool(task.get("requires_bridge", False)),
    }


def _task_inputs(args: argparse.Namespace) -> list[dict[str, Any]]:
    raw_tasks: list[Any] = []
    if args.tasks_file:
        payload = _read_json(Path(args.tasks_file).expanduser().resolve(), "tasks input")
        value = payload.get("tasks")
        if not isinstance(value, list):
            raise StateCtlError("tasks input must contain a tasks list")
        raw_tasks.extend(value)
    for value in args.task_json:
        try:
            raw_tasks.append(json.loads(value))
        except json.JSONDecodeError as exc:
            raise StateCtlError(f"invalid --task-json: {exc}") from exc
    if not raw_tasks:
        if not args.task_title or not args.done_when:
            raise StateCtlError(
                "standalone init requires --task-title and --done-when, or task JSON input"
            )
        raw_tasks.append(
            {
                "id": args.task_id or args.id,
                "title": args.task_title,
                "done_when": args.done_when,
            }
        )
    tasks = [_normalize_task(raw, index) for index, raw in enumerate(raw_tasks)]
    ids = [task["id"] for task in tasks]
    if len(ids) != len(set(ids)):
        raise StateCtlError("standalone task ids must be unique")
    return tasks


def command_route(args: argparse.Namespace) -> dict[str, Any]:
    if args.source not in SOURCE_KINDS:
        raise StateCtlError("source must be standalone or openspec")
    if args.expected_turns < 0 or args.expected_context_kib < 0:
        raise StateCtlError("expected workload values must be non-negative")
    if args.threshold_turns < 1 or args.threshold_context_kib < 1:
        raise StateCtlError("routing thresholds must be positive")
    long_running = (
        args.handoff
        or args.expected_turns >= args.threshold_turns
        or args.expected_context_kib >= args.threshold_context_kib
    )
    if args.profile == "auto" and not long_running:
        return {
            "ok": True,
            "command": "route",
            "decision": "passthrough",
            "source": args.source,
            "adapter": "manual",
            "capabilities": {},
            "reason": "workload is below state-management thresholds; runtime not probed",
            "creates_state": False,
        }
    if args.profile == "lite":
        return {
            "ok": True,
            "command": "route",
            "decision": "lite",
            "source": args.source,
            "adapter": "manual",
            "capabilities": {},
            "reason": "lite profile requested; runtime not probed",
            "creates_state": True,
        }
    probe = probe_runtime(
        args.adapter,
        custom_manifest=args.manifest,
        search_path=args.search_path,
        trust_custom=args.trust_custom_adapter,
    )
    reset_available = supports_reset(probe)
    reason: str
    if args.profile == "reset":
        if reset_available:
            decision = "reset"
            reason = "reset requested and capability confirmed"
        else:
            decision = "lite"
            reason = "reset requested but no reset capability was confirmed"
    elif not long_running:
        decision = "passthrough"
        reason = "workload is below state-management thresholds"
    elif reset_available:
        decision = "reset"
        reason = "long workload and reset capability confirmed"
    else:
        decision = "lite"
        reason = "long workload without a confirmed reset capability"
    return {
        "ok": True,
        "command": "route",
        "decision": decision,
        "source": args.source,
        "adapter": probe["selected"]["id"],
        "capabilities": probe["selected"]["capabilities"],
        "reason": reason,
        "creates_state": decision != "passthrough",
    }


def command_init(args: argparse.Namespace) -> dict[str, Any]:
    root, path = _state_path(args, must_exist=False)
    if path.exists():
        raise StateCtlError(f"state already exists: {path}")
    identifier = _safe_id(args.id)
    goal = _require_text(args.goal, "goal")
    implementation_ref = _require_text(
        args.implementation_ref, "implementation_ref", max_bytes=1024
    )
    constraints = _string_list(args.constraint, "constraints", max_items=32, item_bytes=1024)
    if args.source not in SOURCE_KINDS:
        raise StateCtlError("source must be standalone or openspec")
    if args.profile not in EXECUTION_PROFILES:
        raise StateCtlError("profile must be lite or reset")

    probe = probe_runtime(
        args.adapter,
        custom_manifest=args.manifest,
        search_path=args.search_path,
        trust_custom=args.trust_custom_adapter,
    )
    if args.profile == "reset" and not supports_reset(probe):
        raise StateCtlError(
            "reset profile requested without a confirmed reset capability",
            kind="unsupported_runtime",
            code=ERROR_RUNTIME,
        )

    ledger: dict[str, Any] | None = None
    openspec_tasks: list[dict[str, Any]] | None = None
    if args.source == "standalone":
        tasks = _task_inputs(args)
        tasks[0]["status"] = "in_progress"
        ledger = {"schema_version": TASK_LEDGER_VERSION, "tasks": tasks}
        _validate_task_ledger(ledger)
        source = {"kind": "standalone", "tasks_ref": "tasks.json"}
        active_task: dict[str, Any] | None = _active_from_task(tasks[0], "standalone")
        next_action: str | None = _task_next_action(tasks[0])
        status = "in_progress"
    else:
        if not args.change or not args.tasks_path:
            raise StateCtlError("openspec init requires --change and --tasks-path")
        change = _safe_id(args.change, "change")
        candidate = Path(args.tasks_path)
        tasks_path = candidate if candidate.is_absolute() else root / candidate
        tasks_path = _inside(tasks_path, root, "OpenSpec tasks path")
        _, openspec_tasks = _parse_openspec_tasks(tasks_path)
        source = {
            "kind": "openspec",
            "change": change,
            "tasks_path": _relative_project_path(tasks_path, root),
        }
        pending = next((task for task in openspec_tasks if not task["complete"]), None)
        active_task = _active_from_task(pending, "openspec") if pending else None
        next_action = _task_next_action(pending) if pending else None
        status = "in_progress" if pending else "complete"

    capabilities = dict(probe["selected"]["capabilities"])
    state: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "id": identifier,
        "revision": 0,
        "source": source,
        "execution": {
            "profile": args.profile,
            "adapter": probe["selected"]["id"],
            "capabilities": capabilities,
        },
        "implementation_ref": implementation_ref,
        "goal": goal,
        "status": status,
        "constraints": constraints,
        "active_task": active_task,
        "next_action": next_action,
        "observation": "",
        "blocker": None,
        "artifacts": [],
        "checkpoint": {
            "ready": True,
            "reason": "initial task is ready for handoff" if active_task else "all tasks complete",
            "revision": 0,
        },
        "quality": {
            "invariants": constraints if args.invariant_from_constraints else args.invariant,
            "completed_chunks": 0,
            "review_interval": args.review_interval,
            "review_required": False,
            "review_reasons": [],
            "last_review": None,
            "pending_bridge": False,
            "next_handoff": "continue",
        },
        "worker_lease": None,
    }
    if ledger is not None:
        ledger_path = _task_ledger_path(path)
        _atomic_write_json(ledger_path, ledger)
        try:
            _validate_state(state, path, root)
            _atomic_write_json(path, state)
        except Exception:
            try:
                ledger_path.unlink()
                if not any(ledger_path.parent.iterdir()):
                    ledger_path.parent.rmdir()
            except OSError:
                pass
            raise
    else:
        _validate_state(state, path, root)
        _atomic_write_json(path, state)
    return {
        "ok": True,
        "command": "init",
        "state": str(path),
        "source": args.source,
        "profile": args.profile,
        "adapter": state["execution"]["adapter"],
        "revision": 0,
        "bytes": path.stat().st_size,
    }


def command_begin(args: argparse.Namespace) -> dict[str, Any]:
    root, path, current = _load_state(args)
    _expect_revision(current, args.expected_revision)
    if current.get("worker_lease") is not None:
        raise StateCtlError(
            "cannot begin while a worker lease is active",
            kind="worker_lease_conflict",
            code=ERROR_CONFLICT,
        )
    task_id = _safe_id(args.task_id, "task_id")
    state = copy.deepcopy(current)
    active = state["active_task"]
    if active is not None and active["id"] != task_id:
        raise StateCtlError(f"task {active['id']} is already active")
    if active is not None and active["status"] == "in_progress":
        raise StateCtlError(f"task {task_id} is already active")

    source_kind = state["source"]["kind"]
    ledger_path: Path | None = None
    ledger: dict[str, Any] | None = None
    if source_kind == "standalone":
        ledger_path, current_ledger, tasks = _load_ledger(path)
        ledger = copy.deepcopy(current_ledger)
        ledger_tasks = ledger["tasks"]
        matches = [task for task in ledger_tasks if task["id"] == task_id]
        if not matches:
            raise StateCtlError(f"standalone task not found: {task_id}")
        task = matches[0]
        if task["status"] == "complete":
            raise StateCtlError(f"task {task_id} is already complete")
        task["status"] = "in_progress"
    else:
        tasks_path = _inside(root / state["source"]["tasks_path"], root, "OpenSpec tasks path")
        _, tasks = _parse_openspec_tasks(tasks_path)
        task = _check_openspec_task(tasks, task_id)
        if task["complete"]:
            raise StateCtlError(f"OpenSpec task {task_id} is already checked")

    resumed = active is not None and active["status"] == "blocked"
    if resumed:
        state["active_task"]["status"] = "in_progress"
    else:
        state["active_task"] = _active_from_task(task, source_kind)
    state["status"] = "in_progress"
    state["next_action"] = (
        _require_text(args.next_action, "next_action", max_bytes=1024)
        if args.next_action
        else _task_next_action(task)
    )
    if not resumed:
        state["observation"] = ""
    state["blocker"] = None
    state.pop("last_result", None)
    _advance(state, checkpoint_ready=False, reason="task started")
    _validate_state(state, path, root, ledger_override=ledger)
    if ledger is not None and ledger_path is not None:
        _validate_task_ledger(ledger)
        _write_pair_with_rollback(
            root,
            ledger_path,
            _json_bytes(ledger),
            path,
            _json_bytes(state),
        )
    else:
        _atomic_write_json(path, state)
    return {
        "ok": True,
        "command": "begin",
        "task": task_id,
        "revision": state["revision"],
    }


def command_observe(args: argparse.Namespace) -> dict[str, Any]:
    root, path, current = _load_state(args)
    _expect_revision(current, args.expected_revision)
    if current["active_task"] is None:
        raise StateCtlError("observe requires an active task")
    observation = _require_text(
        args.observation,
        "observation",
        max_bytes=MAX_OBSERVATION_BYTES,
    )
    state = copy.deepcopy(current)
    _release_worker_lease(state, args.run_id)
    state["observation"] = observation
    if args.next_action is not None:
        state["next_action"] = _require_text(
            args.next_action, "next_action", max_bytes=1024
        )
    if args.evidence:
        combined = state["active_task"]["evidence"] + args.evidence
        state["active_task"]["evidence"] = _evidence_list(combined)
    if args.check_json:
        incoming = _parse_checks(args.check_json)
        existing = {check["id"]: check for check in state["active_task"]["checks"]}
        for check in incoming:
            existing[check["id"]] = check
        state["active_task"]["checks"] = _check_list(list(existing.values()))
    if args.artifact:
        artifacts = _string_list(
            state["active_task"]["artifacts"] + args.artifact,
            "active_task.artifacts",
            item_bytes=1024,
        )
        state["active_task"]["artifacts"] = list(dict.fromkeys(artifacts))
        state["artifacts"] = list(
            dict.fromkeys(
                _string_list(state["artifacts"] + args.artifact, "artifacts", item_bytes=1024)
            )
        )
    _advance(state, checkpoint_ready=False, reason="new observation")
    _validate_state(state, path, root)
    _atomic_write_json(path, state)
    return {
        "ok": True,
        "command": "observe",
        "task": state["active_task"]["id"],
        "revision": state["revision"],
    }


def command_block(args: argparse.Namespace) -> dict[str, Any]:
    root, path, current = _load_state(args)
    _expect_revision(current, args.expected_revision)
    if current["active_task"] is None:
        raise StateCtlError("block requires an active task")
    reason = _require_text(args.reason, "reason", max_bytes=MAX_OBSERVATION_BYTES)
    state = copy.deepcopy(current)
    _release_worker_lease(state, args.run_id)
    state["status"] = "blocked"
    state["active_task"]["status"] = "blocked"
    state["blocker"] = reason
    state["observation"] = reason
    state["next_action"] = (
        _require_text(args.next_action, "next_action", max_bytes=1024)
        if args.next_action
        else "Resolve blocker"
    )
    _advance(state, checkpoint_ready=True, reason="blocked state is resumable")

    if state["source"]["kind"] == "standalone":
        ledger_path, ledger, _ = _load_ledger(path)
        next_ledger = copy.deepcopy(ledger)
        for task in next_ledger["tasks"]:
            if task["id"] == state["active_task"]["id"]:
                task["status"] = "blocked"
                break
        _validate_task_ledger(next_ledger)
        _validate_state(state, path, root, ledger_override=next_ledger)
        _write_pair_with_rollback(
            root,
            ledger_path,
            _json_bytes(next_ledger),
            path,
            _json_bytes(state),
        )
    else:
        _validate_state(state, path, root)
        _atomic_write_json(path, state)
    return {
        "ok": True,
        "command": "block",
        "task": state["active_task"]["id"],
        "revision": state["revision"],
        "checkpoint_ready": True,
    }


def command_complete(args: argparse.Namespace) -> dict[str, Any]:
    root, path, current = _load_state(args)
    _expect_revision(current, args.expected_revision)
    if current["active_task"] is None:
        raise StateCtlError("complete requires an active task")
    summary = _require_text(args.summary, "summary", max_bytes=1024)
    evidence = _evidence_list(current["active_task"]["evidence"] + args.evidence)
    incoming_checks = _parse_checks(args.check_json)
    merged_checks = {check["id"]: check for check in current["active_task"]["checks"]}
    for check in incoming_checks:
        merged_checks[check["id"]] = check
    checks = _check_list(list(merged_checks.values()))
    if not evidence and not checks:
        raise StateCtlError("completion requires verification evidence or checks")
    passed = {check["id"] for check in checks if check["status"] == "passed"}
    missing = [
        check_id for check_id in current["active_task"]["regression_checks"]
        if check_id not in passed
    ]
    if missing:
        raise StateCtlError(f"completion missing passed checks: {', '.join(missing)}")
    failed = [check["id"] for check in checks if check["status"] == "failed"]
    if failed:
        raise StateCtlError(f"completion has failed checks: {', '.join(failed)}")
    state = copy.deepcopy(current)
    _release_worker_lease(state, args.run_id)
    task_id = state["active_task"]["id"]
    source_kind = state["source"]["kind"]

    with ExitStack() as locks:
        if source_kind == "openspec":
            tasks_path = _inside(
                root / state["source"]["tasks_path"],
                root,
                "OpenSpec tasks path",
            )
            locks.enter_context(_authority_lock(tasks_path))
            # Revalidate after acquiring the shared authority lock: another
            # controller may have updated tasks.md while this command waited.
            _validate_state(current, path, root)
        return _complete_with_authority_locked(
            args,
            root,
            path,
            state,
            task_id,
            source_kind,
        )


def _complete_with_authority_locked(
    args: argparse.Namespace,
    root: Path,
    path: Path,
    state: dict[str, Any],
    task_id: str,
    source_kind: str,
) -> dict[str, Any]:
    summary = _require_text(args.summary, "summary", max_bytes=1024)
    evidence = _evidence_list(state["active_task"]["evidence"] + args.evidence)
    merged_checks = {check["id"]: check for check in state["active_task"]["checks"]}
    for check in _parse_checks(args.check_json):
        merged_checks[check["id"]] = check
    checks = _check_list(list(merged_checks.values()))

    first_path: Path
    first_payload: bytes
    if source_kind == "standalone":
        ledger_path, current_ledger, _ = _load_ledger(path)
        ledger = copy.deepcopy(current_ledger)
        matching = [task for task in ledger["tasks"] if task["id"] == task_id]
        if not matching:
            raise StateCtlError(f"standalone task disappeared: {task_id}")
        task = matching[0]
        task["status"] = "complete"
        task["evidence"] = evidence
        task["checks"] = checks
        task["summary"] = summary
        remaining = _next_pending(ledger["tasks"])
        if remaining is not None:
            remaining["status"] = "in_progress"
        _validate_task_ledger(ledger)
        first_path = ledger_path
        first_payload = _json_bytes(ledger)
    else:
        tasks_path = _inside(
            root / state["source"]["tasks_path"], root, "OpenSpec tasks path"
        )
        old_text, tasks = _parse_openspec_tasks(tasks_path)
        task = _check_openspec_task(tasks, task_id)
        if task["complete"]:
            raise StateCtlError(f"OpenSpec task {task_id} is already checked")
        new_text = _mark_openspec_complete(old_text, task["line"])
        _, updated_tasks = _parse_openspec_tasks_from_text(new_text)
        remaining = next((item for item in updated_tasks if not item["complete"]), None)
        first_path = tasks_path
        first_payload = new_text.encode("utf-8")

    completed_task = state["active_task"]
    state["last_result"] = {
        "id": task_id,
        "summary": summary,
        "evidence": evidence,
        "checks": checks,
    }
    state["active_task"] = None
    state["blocker"] = None
    state["observation"] = summary
    if remaining is None:
        state["status"] = "complete"
        state["active_task"] = None
        state["next_action"] = None
    else:
        state["status"] = "in_progress"
        state["active_task"] = _active_from_task(remaining, source_kind)
        state["next_action"] = _task_next_action(remaining)
    quality = state["quality"]
    quality["completed_chunks"] += 1
    reasons: list[str] = []
    if quality["completed_chunks"] % quality["review_interval"] == 0:
        reasons.append(f"periodic review after {quality['completed_chunks']} chunks")
    if task_id.startswith("review-recovery-"):
        reasons.append("re-review after architecture recovery")
    if completed_task["requires_bridge"] or len(completed_task["affected_areas"]) > 1:
        quality["pending_bridge"] = True
    if completed_task["kind"] == "integration":
        quality["pending_bridge"] = False
    if quality["pending_bridge"] and remaining is not None and remaining.get("kind", "implementation") != "integration":
        reasons.append("cross-area change requires an integration chunk")
    quality["review_required"] = bool(reasons)
    quality["review_reasons"] = reasons
    next_cohesion = remaining.get("cohesion_key", remaining["id"]) if remaining else None
    if reasons or quality["pending_bridge"]:
        quality["next_handoff"] = "checkpoint"
    elif next_cohesion == completed_task["cohesion_key"]:
        quality["next_handoff"] = "continue"
    elif any(state["execution"]["capabilities"].get(name) for name in ("fresh_context", "in_place_compaction")):
        quality["next_handoff"] = "reset"
    else:
        quality["next_handoff"] = "checkpoint"
    _advance(state, checkpoint_ready=True, reason=f"task {task_id} verified")
    _validate_state(
        state,
        path,
        root,
        ledger_override=ledger if source_kind == "standalone" else None,
        openspec_text_override=new_text if source_kind == "openspec" else None,
    )
    _write_pair_with_rollback(
        root,
        first_path,
        first_payload,
        path,
        _json_bytes(state),
        expected_first=old_text.encode("utf-8") if source_kind == "openspec" else None,
    )
    return {
        "ok": True,
        "command": "complete",
        "task": task_id,
        "revision": state["revision"],
        "all_complete": state["status"] == "complete",
        "checkpoint_ready": True,
        "next_handoff": state["quality"]["next_handoff"],
        "review_required": state["quality"]["review_required"],
    }


def _parse_openspec_tasks_from_text(text: str) -> tuple[str, list[dict[str, Any]]]:
    """Parse already loaded OpenSpec text without creating a temporary file."""

    tasks: list[dict[str, Any]] = []
    seen: set[str] = set()
    for line_number, line_with_ending in enumerate(text.splitlines(keepends=True), start=1):
        line = line_with_ending.rstrip("\r\n")
        match = _TASK_LINE.match(line)
        if not match:
            continue
        task_id = match.group("id")
        if task_id in seen:
            raise StateCtlError(f"duplicate OpenSpec task id {task_id!r}")
        seen.add(task_id)
        tasks.append(
            {
                "id": task_id,
                "title": match.group("title").strip(),
                "complete": match.group("mark").lower() == "x",
                "line": line_number,
            }
        )
    if not tasks:
        raise StateCtlError("OpenSpec tasks.md has no executable tasks")
    return text, tasks


def command_checkpoint(args: argparse.Namespace) -> dict[str, Any]:
    root, path, current = _load_state(args)
    _expect_revision(current, args.expected_revision)
    if current.get("worker_lease") is not None:
        raise StateCtlError(
            "cannot checkpoint while a worker lease is active",
            kind="worker_lease_conflict",
            code=ERROR_CONFLICT,
        )
    state = copy.deepcopy(current)
    if state["active_task"] is not None and state["status"] == "in_progress":
        if not state["observation"].strip():
            raise StateCtlError("checkpoint requires an observation for active work")
        if not state["next_action"]:
            raise StateCtlError("checkpoint requires a next action for active work")
    if state["status"] == "blocked" and not state["blocker"]:
        raise StateCtlError("checkpoint requires the blocker reason")
    reason = (
        _require_text(args.reason, "reason", max_bytes=1024)
        if args.reason
        else "state is resumable"
    )
    _advance(state, checkpoint_ready=True, reason=reason)
    _validate_state(state, path, root)
    _atomic_write_json(path, state)
    return {
        "ok": True,
        "command": "checkpoint",
        "revision": state["revision"],
        "ready": True,
    }


def command_architecture_review(args: argparse.Namespace) -> dict[str, Any]:
    root, path, current = _load_state(args)
    _expect_revision(current, args.expected_revision)
    if current.get("worker_lease") is not None:
        raise StateCtlError("cannot review architecture while a worker lease is active")
    review = _parse_architecture_review(args.review_json)
    compact_review = _compact_architecture_review(review)
    state = copy.deepcopy(current)
    state["quality"]["last_review"] = compact_review
    recovery_task: dict[str, Any] | None = None
    if review["verdict"] == "passed":
        state["quality"]["review_required"] = False
        state["quality"]["review_reasons"] = []
        if state["quality"]["pending_bridge"]:
            state["quality"]["next_handoff"] = "checkpoint"
        elif (
            state["source"]["kind"] == "standalone"
            and state.get("active_task") is not None
            and state.get("last_result", {}).get("id", "").startswith("review-recovery-")
        ):
            _, _, ledger_tasks = _load_ledger(path)
            recovery_index = next(
                (
                    index for index, task in enumerate(ledger_tasks)
                    if task["id"] == state["last_result"]["id"]
                ),
                None,
            )
            previous = next(
                (
                    task for task in reversed(ledger_tasks[:recovery_index])
                    if task["status"] == "complete" and not task["id"].startswith("review-recovery-")
                ),
                None,
            ) if recovery_index is not None else None
            if previous and previous["cohesion_key"] == state["active_task"]["cohesion_key"]:
                state["quality"]["next_handoff"] = "continue"
            elif any(
                state["execution"]["capabilities"].get(name)
                for name in ("fresh_context", "in_place_compaction")
            ):
                state["quality"]["next_handoff"] = "reset"
            else:
                state["quality"]["next_handoff"] = "checkpoint"
        _advance(state, checkpoint_ready=True, reason="architecture review passed")
        _validate_state(state, path, root)
        _atomic_write_json(path, state)
    elif state["source"]["kind"] == "standalone":
        ledger_path, current_ledger, _ = _load_ledger(path)
        ledger = copy.deepcopy(current_ledger)
        active = state.get("active_task")
        if active is None:
            active_index = len(ledger["tasks"])
        else:
            active_id = active["id"]
            active_index = next(
                (index for index, task in enumerate(ledger["tasks"]) if task["id"] == active_id),
                None,
            )
            if active_index is None:
                raise StateCtlError("active task is absent from standalone task ledger")
            ledger["tasks"][active_index]["status"] = "pending"
        recovery_id = f"review-recovery-{current['revision'] + 1}"
        if any(task["id"] == recovery_id for task in ledger["tasks"]):
            raise StateCtlError(f"recovery task already exists: {recovery_id}")
        blocker_contracts = [item["contract"] for item in review["blockers"]]
        affected_files = list(dict.fromkeys(
            path_value
            for blocker in review["blockers"]
            for path_value in blocker["affected_files"]
        ))
        regression_checks = list(dict.fromkeys(
            blocker["regression_check"] for blocker in review["blockers"]
        ))
        recovery_task = _normalize_task(
            {
                "id": recovery_id,
                "title": "Resolve architecture review blockers",
                "done_when": blocker_contracts,
                "kind": "architecture",
                "cohesion_key": "review-recovery",
                "affected_areas": ["architecture"],
                "reads": affected_files,
                "writes": affected_files,
                "contracts": blocker_contracts,
                "regression_checks": regression_checks,
            },
            active_index,
        )
        recovery_task["status"] = "in_progress"
        ledger["tasks"].insert(active_index, recovery_task)
        state["status"] = "in_progress"
        state["active_task"] = _active_from_task(recovery_task, "standalone")
        state["next_action"] = _task_next_action(recovery_task)
        state["blocker"] = None
        state["quality"]["review_required"] = False
        state["quality"]["review_reasons"] = []
        state["quality"]["next_handoff"] = "checkpoint"
        _advance(state, checkpoint_ready=True, reason="architecture recovery task created")
        _validate_task_ledger(ledger)
        _validate_state(state, path, root, ledger_override=ledger)
        _write_pair_with_rollback(
            root,
            ledger_path,
            _json_bytes(ledger),
            path,
            _json_bytes(state),
        )
    else:
        state["status"] = "blocked"
        state["active_task"]["status"] = "blocked"
        state["blocker"] = review["blockers"][0]["contract"]
        state["next_action"] = "Resolve architecture review blockers"
        state["quality"]["review_required"] = True
        state["quality"]["review_reasons"] = ["architecture review blockers remain"]
        state["quality"]["next_handoff"] = "checkpoint"
        _advance(state, checkpoint_ready=True, reason="architecture review blocked")
        _validate_state(state, path, root)
        _atomic_write_json(path, state)
    return {
        "ok": True,
        "command": "architecture-review",
        "revision": state["revision"],
        "verdict": review["verdict"],
        "recovery_task": recovery_task,
        "pending_bridge": state["quality"]["pending_bridge"],
    }


def _validate_context_map(value: dict[str, Any], root: Path) -> list[dict[str, Any]]:
    if set(value) != {"schema_version", "state_id", "revision", "entries"}:
        raise StateCtlError("context map fields are invalid")
    if value.get("schema_version") != CONTEXT_MAP_VERSION:
        raise StateCtlError(f"context map schema_version must equal {CONTEXT_MAP_VERSION}")
    _safe_id(value.get("state_id"), "context map state_id")
    revision = value.get("revision")
    if not isinstance(revision, int) or isinstance(revision, bool) or revision < 0:
        raise StateCtlError("context map revision is invalid")
    entries = value.get("entries")
    if not isinstance(entries, list) or len(entries) > 64:
        raise StateCtlError("context map entries must contain at most 64 items")
    seen: set[str] = set()
    for index, entry in enumerate(entries):
        if not isinstance(entry, dict) or set(entry) != {"path", "purpose", "areas", "symbols", "updated_revision"}:
            raise StateCtlError(f"context map entries[{index}] fields are invalid")
        relative = _require_text(entry.get("path"), f"context map entries[{index}].path", max_bytes=256)
        _inside(root / relative, root, "context map entry path")
        if relative in seen:
            raise StateCtlError(f"duplicate context map path: {relative}")
        seen.add(relative)
        _require_text(entry.get("purpose"), f"context map entries[{index}].purpose", max_bytes=512)
        _string_list(entry.get("areas"), f"context map entries[{index}].areas", max_items=8, item_bytes=128)
        _string_list(entry.get("symbols"), f"context map entries[{index}].symbols", max_items=12, item_bytes=128)
        if not isinstance(entry.get("updated_revision"), int) or isinstance(entry["updated_revision"], bool) or entry["updated_revision"] < 0:
            raise StateCtlError(f"context map entries[{index}].updated_revision is invalid")
    if len(_json_bytes(value)) > MAX_TASK_LEDGER_BYTES:
        raise StateCtlError(f"context map exceeds {MAX_TASK_LEDGER_BYTES} bytes")
    return entries


def command_context_map_update(args: argparse.Namespace) -> dict[str, Any]:
    root, path, state = _load_state(args)
    _expect_revision(state, args.expected_revision)
    if state.get("worker_lease") is not None:
        raise StateCtlError("cannot update context map while a worker lease is active")
    try:
        raw = json.loads(args.entry_json)
    except json.JSONDecodeError as exc:
        raise StateCtlError(f"invalid --entry-json: {exc}") from exc
    if not isinstance(raw, dict) or set(raw) != {"path", "purpose", "areas", "symbols"}:
        raise StateCtlError("context map entry fields must be path, purpose, areas, symbols")
    relative = _require_text(raw.get("path"), "context map entry path", max_bytes=256)
    candidate = _inside(root / relative, root, "context map entry path")
    relative = candidate.relative_to(root).as_posix()
    entry = {
        "path": relative,
        "purpose": _require_text(raw.get("purpose"), "context map entry purpose", max_bytes=512),
        "areas": _string_list(raw.get("areas"), "context map entry areas", max_items=8, item_bytes=128),
        "symbols": _string_list(raw.get("symbols"), "context map entry symbols", max_items=12, item_bytes=128),
        "updated_revision": state["revision"],
    }
    map_path = _context_map_path(path)
    if map_path.exists():
        context_map = _read_json(map_path, "context map")
        entries = _validate_context_map(context_map, root)
        if context_map["state_id"] != state["id"]:
            raise StateCtlError("context map belongs to another state")
        next_entries = [existing for existing in entries if existing["path"] != relative]
    else:
        next_entries = []
    next_entries.append(entry)
    context_map = {
        "schema_version": CONTEXT_MAP_VERSION,
        "state_id": state["id"],
        "revision": state["revision"],
        "entries": sorted(next_entries, key=lambda item: item["path"]),
    }
    _validate_context_map(context_map, root)
    _atomic_write_json(map_path, context_map)
    return {"ok": True, "command": "context-map-update", "path": relative, "entries": len(next_entries)}


def _relevant_context(state: dict[str, Any], state_path: Path, root: Path) -> list[dict[str, Any]]:
    map_path = _context_map_path(state_path)
    if not map_path.exists() or state["active_task"] is None:
        return []
    context_map = _read_json(map_path, "context map")
    entries = _validate_context_map(context_map, root)
    if context_map["state_id"] != state["id"]:
        raise StateCtlError("context map belongs to another state")
    active = state["active_task"]
    paths = set(active["reads"] + active["writes"] + active["artifacts"])
    areas = set(active["affected_areas"])
    relevant = [entry for entry in entries if entry["path"] in paths or areas.intersection(entry["areas"])]
    return relevant[:16]


def _active_packet_task(
    state: dict[str, Any], state_path: Path, root: Path
) -> dict[str, Any] | None:
    active = state["active_task"]
    if active is None:
        return None
    packet_task = {
        "id": active["id"],
        "title": active["title"],
        "done_when": active["done_when"],
        "source_refs": [],
        "working_files": active["artifacts"],
        "kind": active["kind"],
        "cohesion_key": active["cohesion_key"],
        "affected_areas": active["affected_areas"],
        "reads": active["reads"],
        "writes": active["writes"],
        "contracts": active["contracts"],
        "regression_checks": active["regression_checks"],
    }
    if state["source"]["kind"] == "openspec":
        packet_task["source_refs"] = [f"{state['source']['tasks_path']}#{active['id']}"]
    else:
        packet_task["source_refs"] = ["user-prompt"]
    return packet_task


def command_packet(args: argparse.Namespace) -> dict[str, Any]:
    root, path, current = _load_state(args)
    if args.expected_revision is not None:
        _expect_revision(current, args.expected_revision)
    if current.get("worker_lease") is not None:
        raise StateCtlError(
            "a worker lease is already active",
            kind="worker_lease_conflict",
            code=ERROR_CONFLICT,
        )
    if not current["checkpoint"]["ready"]:
        raise StateCtlError("packet requires a ready checkpoint")
    if current["active_task"] is None:
        raise StateCtlError("worker packet requires an active task")
    if current["active_task"]["status"] != "in_progress":
        raise StateCtlError("worker packet requires an in-progress task")
    if current["quality"]["review_required"]:
        raise StateCtlError("architecture review is required before the next worker packet")
    if current["quality"]["pending_bridge"] and current["active_task"]["kind"] != "integration":
        raise StateCtlError("an integration chunk is required before further implementation")
    if args.max_turns < 1:
        raise StateCtlError("max_turns must be positive")
    if args.max_result_bytes < 1024 or args.max_result_bytes > 64 * 1024:
        raise StateCtlError("max_result_bytes must be between 1024 and 65536")
    run_id = _safe_id(args.run_id, "run_id") if args.run_id else uuid.uuid4().hex
    state = copy.deepcopy(current)
    _advance(state, checkpoint_ready=True, reason=f"worker lease {run_id}")
    state["worker_lease"] = {
        "run_id": run_id,
        "task_id": state["active_task"]["id"],
        "based_on_revision": state["revision"],
    }
    _validate_state(state, path, root)
    packet_source = {"kind": state["source"]["kind"]}
    if state["source"]["kind"] == "openspec":
        packet_source["change"] = state["source"]["change"]
    packet: dict[str, Any] = {
        "protocol": WORKER_PROTOCOL,
        "packet_version": PACKET_VERSION,
        "run_id": run_id,
        "state_id": state["id"],
        "based_on_revision": state["revision"],
        "language_policy": {
            "operational": "en",
            "source_content": "preserve",
        },
        "source": packet_source,
        "goal": state["goal"],
        "implementation_ref": state["implementation_ref"],
        "constraints": state["constraints"],
        "task": _active_packet_task(state, path, root),
        "next_action": state["next_action"],
        "last_observation": state["observation"],
        "quality": {
            "invariants": state["quality"]["invariants"],
            "next_handoff": state["quality"]["next_handoff"],
        },
        "context": _relevant_context(state, path, root),
        "limits": {
            "max_turns": args.max_turns,
            "max_result_bytes": args.max_result_bytes,
        },
    }
    payload = _json_bytes(packet)
    if len(payload) > MAX_PACKET_BYTES:
        raise StateCtlError(f"worker packet exceeds {MAX_PACKET_BYTES} bytes")
    output_candidate = Path(args.output).expanduser()
    output = output_candidate if output_candidate.is_absolute() else root / output_candidate
    output = _inside(output, root, "packet output")
    transaction_marker, transaction_backup = _transaction_paths(path)
    protected = {path, transaction_marker, transaction_backup}
    if state["source"]["kind"] == "standalone":
        protected.add(_task_ledger_path(path).resolve())
    else:
        authority_path = _inside(
            root / state["source"]["tasks_path"],
            root,
            "OpenSpec tasks path",
        )
        protected.add(authority_path)
        protected.add(_authority_lock_path(authority_path))
    protected.add(_context_map_path(path).resolve())
    if output in protected:
        raise StateCtlError("packet output must not overwrite state or an authority file")
    lock_path = path.parent / ".statectl.lock"
    if output == lock_path or lock_path in output.parents:
        raise StateCtlError("packet output must not be inside the controller lock")
    _atomic_create_bytes(output, payload)
    try:
        _atomic_write_json(path, state)
    except Exception:
        try:
            output.unlink()
        except OSError:
            pass
        raise
    return {
        "ok": True,
        "command": "packet",
        "output": str(output),
        "run_id": run_id,
        "revision": state["revision"],
        "bytes": len(payload),
    }


def command_validate(args: argparse.Namespace) -> dict[str, Any]:
    _, path, state = _load_state(args)
    return {
        "ok": True,
        "command": "validate",
        "state": str(path),
        "revision": state["revision"],
        "bytes": path.stat().st_size,
    }


def command_version(args: argparse.Namespace) -> dict[str, Any]:
    del args
    return {"ok": True, "command": "version", **_release_info()}


def command_runtime_probe(args: argparse.Namespace) -> dict[str, Any]:
    probe = probe_runtime(
        args.adapter,
        custom_manifest=args.manifest,
        search_path=args.search_path,
        trust_custom=args.trust_custom_adapter,
    )
    return {"ok": True, "command": "runtime-probe", **probe}


def _validate_worker_packet(packet: dict[str, Any]) -> None:
    required = {
        "protocol",
        "packet_version",
        "run_id",
        "state_id",
        "based_on_revision",
        "language_policy",
        "source",
        "goal",
        "implementation_ref",
        "constraints",
        "task",
        "next_action",
        "last_observation",
        "limits",
        "quality",
        "context",
    }
    missing = sorted(required - set(packet))
    if missing:
        raise StateCtlError(f"worker packet missing fields: {', '.join(missing)}")
    extra = sorted(set(packet) - required)
    if extra:
        raise StateCtlError(f"worker packet has unsupported fields: {', '.join(extra)}")
    if packet.get("protocol") != WORKER_PROTOCOL:
        raise StateCtlError(f"worker packet protocol must be {WORKER_PROTOCOL}")
    if packet.get("packet_version") != PACKET_VERSION:
        raise StateCtlError(f"worker packet packet_version must equal {PACKET_VERSION}")
    _safe_id(packet.get("run_id"), "worker packet run_id")
    _safe_id(packet.get("state_id"), "worker packet state_id")
    revision = packet.get("based_on_revision")
    if not isinstance(revision, int) or isinstance(revision, bool) or revision < 0:
        raise StateCtlError("worker packet based_on_revision is invalid")
    if packet.get("language_policy") != {
        "operational": "en",
        "source_content": "preserve",
    }:
        raise StateCtlError("worker packet language_policy is invalid")
    source = packet.get("source")
    if not isinstance(source, dict) or source.get("kind") not in SOURCE_KINDS:
        raise StateCtlError("worker packet source is invalid")
    expected_source_fields = (
        {"kind"} if source["kind"] == "standalone" else {"kind", "change"}
    )
    if set(source) != expected_source_fields:
        raise StateCtlError("worker packet source fields are invalid")
    if source["kind"] == "openspec":
        _safe_id(source.get("change"), "worker packet source.change")
    _require_text(packet.get("goal"), "worker packet goal")
    _require_text(
        packet.get("implementation_ref"),
        "worker packet implementation_ref",
        max_bytes=1024,
    )
    _string_list(
        packet.get("constraints"),
        "worker packet constraints",
        max_items=32,
        item_bytes=1024,
    )
    task = packet.get("task")
    if not isinstance(task, dict) or set(task) != {
        "id",
        "title",
        "done_when",
        "source_refs",
        "working_files",
        "kind",
        "cohesion_key",
        "affected_areas",
        "reads",
        "writes",
        "contracts",
        "regression_checks",
    }:
        raise StateCtlError("worker packet task fields are invalid")
    _safe_id(task.get("id"), "worker packet task.id")
    _require_text(task.get("title"), "worker packet task.title", max_bytes=1024)
    if not _string_list(
        task.get("done_when"),
        "worker packet task.done_when",
        max_items=8,
        item_bytes=1024,
    ):
        raise StateCtlError("worker packet task.done_when must not be empty")
    if not _string_list(
        task.get("source_refs"),
        "worker packet task.source_refs",
        max_items=16,
        item_bytes=1024,
    ):
        raise StateCtlError("worker packet task.source_refs must not be empty")
    _string_list(
        task.get("working_files"),
        "worker packet task.working_files",
        max_items=32,
        item_bytes=1024,
    )
    if task.get("kind") not in TASK_KINDS:
        raise StateCtlError("worker packet task.kind is invalid")
    _safe_id(task.get("cohesion_key"), "worker packet task.cohesion_key")
    _string_list(task.get("affected_areas"), "worker packet task.affected_areas", max_items=8, item_bytes=128)
    _string_list(task.get("reads"), "worker packet task.reads", max_items=16, item_bytes=256)
    _string_list(task.get("writes"), "worker packet task.writes", max_items=16, item_bytes=256)
    _string_list(task.get("contracts"), "worker packet task.contracts", max_items=8, item_bytes=512)
    _string_list(task.get("regression_checks"), "worker packet task.regression_checks", max_items=8, item_bytes=128)
    _require_text(packet.get("next_action"), "worker packet next_action", max_bytes=1024)
    observation = packet.get("last_observation")
    if not isinstance(observation, str) or _byte_len(observation) > MAX_OBSERVATION_BYTES:
        raise StateCtlError("worker packet last_observation is invalid")
    quality = packet.get("quality")
    if not isinstance(quality, dict) or set(quality) != {"invariants", "next_handoff"}:
        raise StateCtlError("worker packet quality fields are invalid")
    _string_list(quality.get("invariants"), "worker packet quality.invariants", max_items=16, item_bytes=512)
    if quality.get("next_handoff") not in {"continue", "reset", "checkpoint"}:
        raise StateCtlError("worker packet quality.next_handoff is invalid")
    context = packet.get("context")
    if not isinstance(context, list) or len(context) > 16:
        raise StateCtlError("worker packet context is invalid")
    for index, entry in enumerate(context):
        if not isinstance(entry, dict) or set(entry) != {"path", "purpose", "areas", "symbols", "updated_revision"}:
            raise StateCtlError(f"worker packet context[{index}] fields are invalid")
        _require_text(entry.get("path"), f"worker packet context[{index}].path", max_bytes=256)
        _require_text(entry.get("purpose"), f"worker packet context[{index}].purpose", max_bytes=512)
        _string_list(entry.get("areas"), f"worker packet context[{index}].areas", max_items=8, item_bytes=128)
        _string_list(entry.get("symbols"), f"worker packet context[{index}].symbols", max_items=12, item_bytes=128)
        updated_revision = entry.get("updated_revision")
        if not isinstance(updated_revision, int) or isinstance(updated_revision, bool) or updated_revision < 0:
            raise StateCtlError(f"worker packet context[{index}].updated_revision is invalid")
    limits = packet.get("limits")
    if not isinstance(limits, dict) or set(limits) != {"max_turns", "max_result_bytes"}:
        raise StateCtlError("worker packet limits fields are invalid")
    if (
        not isinstance(limits.get("max_turns"), int)
        or isinstance(limits["max_turns"], bool)
        or limits["max_turns"] < 1
    ):
        raise StateCtlError("worker packet max_turns must be positive")
    if (
        not isinstance(limits.get("max_result_bytes"), int)
        or isinstance(limits["max_result_bytes"], bool)
        or limits["max_result_bytes"] < 1024
        or limits["max_result_bytes"] > 64 * 1024
    ):
        raise StateCtlError("worker packet max_result_bytes is invalid")
    banned = _contains_banned_key(packet)
    if banned:
        raise StateCtlError(f"worker packet must not contain {banned!r}")


def command_runtime_plan(args: argparse.Namespace) -> dict[str, Any]:
    requested = args.adapter
    state_revision: int | None = None
    state_id: str | None = None
    active_task_id: str | None = None
    if not args.state and not args.id:
        raise StateCtlError("runtime-plan requires --state or --id for packet binding")
    root, state_path, state = _load_state(args)
    state_revision = state["revision"]
    state_id = state["id"]
    active_task_id = state["active_task"]["id"] if state["active_task"] else None
    if not state["checkpoint"]["ready"]:
        raise StateCtlError("runtime-plan requires a ready checkpoint")
    lease = state.get("worker_lease")
    if lease is None:
        raise StateCtlError("runtime-plan requires an active worker lease from packet")
    if requested == "auto" and not os.environ.get("EXECUTION_STATE_ADAPTER"):
        requested = state["execution"]["adapter"]
    packet = Path(args.packet).expanduser().resolve()
    if not packet.is_file():
        raise StateCtlError(f"packet file not found: {packet}")
    if packet.stat().st_size > MAX_PACKET_BYTES:
        raise StateCtlError(f"worker packet exceeds {MAX_PACKET_BYTES} bytes")
    packet_data = _read_json(packet, "worker packet")
    _validate_worker_packet(packet_data)
    if packet_data.get("run_id") != lease["run_id"]:
        raise StateCtlError("worker packet run_id does not match active worker lease")
    if state_revision is not None and packet_data.get("based_on_revision") != state_revision:
        raise StateCtlError("worker packet revision does not match state")
    if state_id is not None and packet_data.get("state_id") != state_id:
        raise StateCtlError("worker packet state_id does not match state")
    if state_id is not None and active_task_id is None:
        raise StateCtlError("state has no active task for worker plan")
    if active_task_id is not None and packet_data["task"].get("id") != active_task_id:
        raise StateCtlError("worker packet task does not match active state task")
    expected_source = {"kind": state["source"]["kind"]}
    if state["source"]["kind"] == "openspec":
        expected_source["change"] = state["source"]["change"]
    expected_fields = {
        "source": expected_source,
        "goal": state["goal"],
        "implementation_ref": state["implementation_ref"],
        "constraints": state["constraints"],
        "task": _active_packet_task(state, state_path, root),
        "next_action": state["next_action"],
        "last_observation": state["observation"],
        "quality": {
            "invariants": state["quality"]["invariants"],
            "next_handoff": state["quality"]["next_handoff"],
        },
        "context": _relevant_context(state, state_path, root),
    }
    for field, expected in expected_fields.items():
        if packet_data.get(field) != expected:
            raise StateCtlError(f"worker packet {field} does not match state")
    plan = build_runtime_plan(
        requested=requested,
        prompt_path=args.prompt,
        packet_path=packet,
        project_root=args.project_root,
        custom_manifest=args.manifest,
        search_path=args.search_path,
        trust_custom=args.trust_custom_adapter,
    )
    return {"ok": True, "command": "runtime-plan", **plan}


def _add_state_location(parser: argparse.ArgumentParser, *, id_optional: bool = True) -> None:
    parser.add_argument("--state")
    parser.add_argument("--id", required=not id_optional)
    parser.add_argument("--project-root", default=".")


def _add_runtime_options(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("--adapter", default="auto")
    parser.add_argument("--manifest")
    parser.add_argument("--search-path")
    parser.add_argument("--trust-custom-adapter", action="store_true")


def build_parser() -> argparse.ArgumentParser:
    parser = JsonArgumentParser(prog="statectl")
    subparsers = parser.add_subparsers(dest="command", required=True)

    route = subparsers.add_parser("route")
    route.add_argument("--source", choices=sorted(SOURCE_KINDS), default="standalone")
    route.add_argument("--profile", choices=["auto", "lite", "reset"], default="auto")
    route.add_argument("--expected-turns", type=int, default=1)
    route.add_argument("--expected-context-kib", type=int, default=0)
    route.add_argument("--threshold-turns", type=int, default=8)
    route.add_argument("--threshold-context-kib", type=int, default=64)
    route.add_argument("--handoff", action="store_true")
    _add_runtime_options(route)
    route.set_defaults(handler=command_route)

    init = subparsers.add_parser("init")
    _add_state_location(init, id_optional=False)
    init.add_argument("--source", choices=sorted(SOURCE_KINDS), required=True)
    init.add_argument("--profile", choices=sorted(EXECUTION_PROFILES), default="lite")
    init.add_argument("--implementation-ref", required=True)
    init.add_argument("--goal", required=True)
    init.add_argument("--constraint", action="append", default=[])
    init.add_argument("--invariant", action="append", default=[])
    init.add_argument("--invariant-from-constraints", action="store_true")
    init.add_argument("--review-interval", type=int, default=8)
    init.add_argument("--task-id")
    init.add_argument("--task-title")
    init.add_argument("--done-when", action="append", default=[])
    init.add_argument("--task-json", action="append", default=[])
    init.add_argument("--tasks-file")
    init.add_argument("--change")
    init.add_argument("--tasks-path")
    _add_runtime_options(init)
    init.set_defaults(handler=_locked_init_command(command_init))

    for name, handler in (
        ("begin", command_begin),
        ("observe", command_observe),
        ("complete", command_complete),
        ("block", command_block),
        ("checkpoint", command_checkpoint),
    ):
        command = subparsers.add_parser(name)
        _add_state_location(command)
        command.add_argument("--expected-revision", type=int, required=True)
        command.set_defaults(handler=_locked_state_command(handler))
        if name == "begin":
            command.add_argument("--task-id", required=True)
            command.add_argument("--next-action")
        elif name == "observe":
            command.add_argument("--run-id")
            command.add_argument("--observation", required=True)
            command.add_argument("--next-action")
            command.add_argument("--evidence", action="append", default=[])
            command.add_argument("--check-json", action="append", default=[])
            command.add_argument("--artifact", action="append", default=[])
        elif name == "complete":
            command.add_argument("--run-id")
            command.add_argument("--summary", required=True)
            command.add_argument("--evidence", action="append", default=[])
            command.add_argument("--check-json", action="append", default=[])
        elif name == "block":
            command.add_argument("--run-id")
            command.add_argument("--reason", required=True)
            command.add_argument("--next-action")
        elif name == "checkpoint":
            command.add_argument("--reason")

    packet = subparsers.add_parser("packet")
    _add_state_location(packet)
    packet.add_argument("--output", required=True)
    packet.add_argument("--run-id")
    packet.add_argument("--expected-revision", type=int)
    packet.add_argument("--max-turns", type=int, default=8)
    packet.add_argument("--max-result-bytes", type=int, default=8192)
    packet.set_defaults(handler=_locked_state_command(command_packet))

    architecture_review = subparsers.add_parser("architecture-review")
    _add_state_location(architecture_review)
    architecture_review.add_argument("--expected-revision", type=int, required=True)
    architecture_review.add_argument("--review-json", required=True)
    architecture_review.set_defaults(handler=_locked_state_command(command_architecture_review))

    context_map = subparsers.add_parser("context-map-update")
    _add_state_location(context_map)
    context_map.add_argument("--expected-revision", type=int, required=True)
    context_map.add_argument("--entry-json", required=True)
    context_map.set_defaults(handler=_locked_state_command(command_context_map_update))

    validate = subparsers.add_parser("validate")
    _add_state_location(validate)
    validate.set_defaults(handler=_locked_state_command(command_validate))

    version = subparsers.add_parser("version")
    version.set_defaults(handler=command_version)

    runtime_probe = subparsers.add_parser("runtime-probe")
    _add_runtime_options(runtime_probe)
    runtime_probe.set_defaults(handler=command_runtime_probe)

    runtime_plan = subparsers.add_parser("runtime-plan")
    _add_state_location(runtime_plan)
    _add_runtime_options(runtime_plan)
    runtime_plan.add_argument("--prompt", required=True)
    runtime_plan.add_argument("--packet", required=True)
    runtime_plan.set_defaults(handler=_locked_state_command(command_runtime_plan))
    return parser


def main(argv: list[str] | None = None) -> int:
    try:
        args = build_parser().parse_args(argv)
        result = args.handler(args)
        _emit(result)
        return 0
    except (StateCtlError, AdapterError) as exc:
        if isinstance(exc, StateCtlError):
            kind = exc.kind
            code = exc.code
        else:
            kind = "invalid_adapter"
            code = ERROR_INVALID
        _emit({"ok": False, "error": kind, "message": str(exc)})
        return code
    except OSError as exc:
        _emit({"ok": False, "error": "io_error", "message": str(exc)})
        return ERROR_INVALID


if __name__ == "__main__":
    raise SystemExit(main())
