// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ProjectDocumentDiscussionPreview } from "./projectDocumentDiscussionPreview";

const originalCreateObjectUrl = URL.createObjectURL;
const originalRevokeObjectUrl = URL.revokeObjectURL;

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  if (originalCreateObjectUrl) URL.createObjectURL = originalCreateObjectUrl;
  else Reflect.deleteProperty(URL, "createObjectURL");
  if (originalRevokeObjectUrl) URL.revokeObjectURL = originalRevokeObjectUrl;
  else Reflect.deleteProperty(URL, "revokeObjectURL");
});

describe("ProjectDocumentDiscussionPreview", () => {
  it("loads a PDF with the authenticated API and opens an accessible fullscreen viewer", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(new Blob(["pdf"], { type: "application/pdf" }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    URL.createObjectURL = vi.fn(() => "blob:http://localhost/authorized-pdf");
    URL.revokeObjectURL = vi.fn();

    render(<ProjectDocumentDiscussionPreview documentId="document/1" name="План.pdf" projectId="project/1" />);
    expect(screen.getByLabelText("Загрузка документа «План.pdf»")).toBeTruthy();
    expect((await screen.findByTitle("Предпросмотр документа «План.pdf»")).getAttribute("src")).toContain("blob:http://localhost/authorized-pdf");
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/projects/project%2F1/documents/document%2F1", expect.objectContaining({ credentials: "same-origin" }));
    fireEvent.click(screen.getByRole("button", { name: "Открыть документ «План.pdf» на весь экран" }));
    expect(screen.getByRole("dialog", { name: "Полноэкранный просмотр «План.pdf»" })).toBeTruthy();
    expect(screen.getByTitle("Полноэкранный просмотр документа «План.pdf»")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Закрыть полноэкранный просмотр" }).className).toContain("size-8");
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("loads an image with the authenticated API and reuses its blob URL in fullscreen", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(new Blob(["image"], { type: "image/jpeg" }), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    URL.createObjectURL = vi.fn(() => "blob:http://localhost/authorized-image");
    URL.revokeObjectURL = vi.fn();

    render(<ProjectDocumentDiscussionPreview documentId="image/1" name="Фото.jpg" projectId="project/1" />);
    expect(screen.getByLabelText("Загрузка документа «Фото.jpg»")).toBeTruthy();
    expect((await screen.findByAltText("Предпросмотр изображения «Фото.jpg»")).getAttribute("src")).toBe("blob:http://localhost/authorized-image");
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/projects/project%2F1/documents/image%2F1", expect.objectContaining({ credentials: "same-origin" }));

    fireEvent.click(screen.getByRole("button", { name: "Открыть документ «Фото.jpg» на весь экран" }));
    expect(screen.getByAltText("Полноэкранный просмотр изображения «Фото.jpg»").getAttribute("src")).toBe("blob:http://localhost/authorized-image");
    expect(screen.getByRole("link", { name: "Открыть отдельно" }).getAttribute("href")).toBe("blob:http://localhost/authorized-image");
  });

  it("shows a recoverable image error and supports unsupported files", async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(new Response("unauthorized", { status: 401 })).mockResolvedValueOnce(new Response(new Blob(["image"], { type: "image/jpeg" }), { status: 200 })).mockResolvedValue(new Response(new Blob(["sheet"]), { status: 200 }));
    vi.stubGlobal("fetch", fetchMock);
    URL.createObjectURL = vi.fn(() => "blob:http://localhost/retried-image");
    URL.revokeObjectURL = vi.fn();

    const { rerender } = render(<ProjectDocumentDiscussionPreview documentId="image-1" name="Фото.jpg" projectId="project-1" />);
    expect((await screen.findByRole("alert")).textContent).toContain("Не удалось загрузить документ");
    fireEvent.click(screen.getByRole("button", { name: "Повторить" }));
    await waitFor(() => expect(screen.getByAltText("Предпросмотр изображения «Фото.jpg»")).toBeTruthy());
    rerender(<ProjectDocumentDiscussionPreview documentId="sheet-1" name="Смета.xlsx" projectId="project-1" />);
    expect(await screen.findByText("Предпросмотр этого формата недоступен. Откройте файл отдельно.")).toBeTruthy();
  });

  it("uses a safe fallback for a non-error rejection", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue("network failure"));

    render(<ProjectDocumentDiscussionPreview documentId="image-2" name="Фото.png" projectId="project-1" />);

    expect((await screen.findByRole("alert")).textContent).toContain("Повторите попытку.");
    expect(screen.queryByRole("button", { name: "Открыть документ «Фото.png» на весь экран" })).toBeNull();
  });
});
