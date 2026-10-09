// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ProjectDocumentsPage } from "./projectDocumentsPage";

const api = vi.hoisted(() => ({
  deleteProjectDocument: vi.fn(),
  loadProject: vi.fn(),
  loadProjectDocuments: vi.fn(),
  uploadProjectDocument: vi.fn(),
}));
vi.mock("../api/projects", () => api);
vi.mock("next/link", () => ({ default: ({ children, href, ...props }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => <a href={href} {...props}>{children}</a> }));

const project = { id: "project-1", name: "Дом", status: "active" };
const documentItem = { canDelete: true, createdAt: "2026-09-30T10:00:00Z", id: "document/1", mediaType: "application/pdf", name: "План.pdf", projectId: "project-1", sizeBytes: 2048, uploadedByUserId: "user-1", version: 1 };

afterEach(() => { cleanup(); vi.clearAllMocks(); vi.unstubAllGlobals(); });

function arrangeDocuments(items = [documentItem], canUpload = true) {
  api.loadProject.mockResolvedValue(project);
  api.loadProjectDocuments.mockResolvedValue({ canUpload, items });
}

describe("ProjectDocumentsPage", () => {
  it("loads files, uploads a document and deletes it after confirmation", async () => {
    arrangeDocuments();
    api.uploadProjectDocument.mockResolvedValue({ ...documentItem, id: "document-2", name: "Ведомость.xlsx", sizeBytes: 2_000_000 });
    api.deleteProjectDocument.mockResolvedValue(undefined);
    vi.stubGlobal("confirm", vi.fn(() => true));
    render(<ProjectDocumentsPage projectId="project-1" />);
    expect(await screen.findByRole("link", { name: "План.pdf" })).toHaveProperty("href", expect.stringContaining("document%2F1"));
    expect(screen.getByText(/2 КБ/)).toBeTruthy();
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File(["content"], "Ведомость.xlsx", { type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" });
    fireEvent.change(input, { target: { files: [file] } });
    expect(await screen.findByRole("link", { name: "Ведомость.xlsx" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Удалить План.pdf" }));
    await waitFor(() => expect(api.deleteProjectDocument).toHaveBeenCalledWith("project-1", "document/1"));
    expect(screen.queryByRole("link", { name: "План.pdf" })).toBeNull();
  });

  it("renders empty/read-only state and rejects oversized files", async () => {
    arrangeDocuments([], false);
    render(<ProjectDocumentsPage projectId="project-1" />);
    expect(await screen.findByText("Документов пока нет")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Загрузить документ" })).toBeNull();
    cleanup();
    arrangeDocuments([], true);
    render(<ProjectDocumentsPage projectId="project-1" />);
    await screen.findByRole("button", { name: "Загрузить документ" });
    const oversized = new File(["x"], "archive.zip");
    Object.defineProperty(oversized, "size", { value: 11 * 1024 * 1024 });
    fireEvent.change(document.querySelector('input[type="file"]')!, { target: { files: [oversized] } });
    expect(screen.getByRole("alert").textContent).toContain("10 МБ");
    expect(api.uploadProjectDocument).not.toHaveBeenCalled();
  });

  it("shows load, upload and delete errors and supports retry", async () => {
    api.loadProject.mockRejectedValueOnce(new Error("offline"));
    api.loadProjectDocuments.mockRejectedValueOnce(new Error("offline"));
    render(<ProjectDocumentsPage projectId="project-1" />);
    expect((await screen.findByRole("alert")).textContent).toContain("offline");
    arrangeDocuments();
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    expect(await screen.findByRole("link", { name: "План.pdf" })).toBeTruthy();
    api.uploadProjectDocument.mockRejectedValueOnce(new Error("upload failed"));
    fireEvent.change(document.querySelector('input[type="file"]')!, { target: { files: [new File(["x"], "new.pdf")] } });
    expect((await screen.findByRole("alert")).textContent).toContain("upload failed");
    vi.stubGlobal("confirm", vi.fn(() => false));
    fireEvent.click(screen.getByRole("button", { name: "Удалить План.pdf" }));
    expect(api.deleteProjectDocument).not.toHaveBeenCalled();
    vi.stubGlobal("confirm", vi.fn(() => true));
    api.deleteProjectDocument.mockRejectedValueOnce(new Error("delete failed"));
    fireEvent.click(screen.getByRole("button", { name: "Удалить План.pdf" }));
    expect((await screen.findByRole("alert")).textContent).toContain("delete failed");
  });

  it("formats byte and megabyte sizes and hides deletion without permission", async () => {
    arrangeDocuments([
      { ...documentItem, canDelete: false, id: "small", name: "note.txt", sizeBytes: 10 },
      { ...documentItem, canDelete: false, id: "large", name: "archive.zip", sizeBytes: 2 * 1024 * 1024 },
    ]);
    render(<ProjectDocumentsPage projectId="project-1" />);
    expect(await screen.findByText(/10 Б/)).toBeTruthy();
    expect(screen.getByText(/2.0 МБ/)).toBeTruthy();
    expect(screen.queryByRole("button", { name: /Удалить/ })).toBeNull();
  });

  it("uses generic messages for non-Error load, upload and delete failures", async () => {
    api.loadProject.mockRejectedValueOnce("offline");
    api.loadProjectDocuments.mockRejectedValueOnce("offline");
    render(<ProjectDocumentsPage projectId="project-1" />);
    expect((await screen.findByRole("alert")).textContent).toContain("Не удалось загрузить документы");
    arrangeDocuments();
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    await screen.findByRole("link", { name: "План.pdf" });
    api.uploadProjectDocument.mockRejectedValueOnce("offline");
    fireEvent.change(document.querySelector('input[type="file"]')!, { target: { files: [new File(["x"], "new.pdf")] } });
    expect((await screen.findByRole("alert")).textContent).toContain("Не удалось загрузить документ");
    vi.stubGlobal("confirm", vi.fn(() => true));
    api.deleteProjectDocument.mockRejectedValueOnce("offline");
    fireEvent.click(screen.getByRole("button", { name: "Удалить План.pdf" }));
    expect((await screen.findByRole("alert")).textContent).toContain("Не удалось удалить документ");
  });
});
