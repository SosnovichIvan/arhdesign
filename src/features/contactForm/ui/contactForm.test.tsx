// @vitest-environment jsdom

import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ContactForm } from "./contactForm";

describe("ContactForm", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("shows validation errors without sending an empty form", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    render(<ContactForm />);

    fireEvent.click(screen.getByRole("button", { name: "Обсудить проект" }));
    fireEvent.click(screen.getByRole("button", { name: "Отправить запрос" }));

    expect(await screen.findByText("Укажите имя")).toBeTruthy();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("shows cooldown after a 429 response", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { headers: { "Retry-After": "3600" }, status: 429 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<ContactForm />);

    fireEvent.click(screen.getByRole("button", { name: "Обсудить проект" }));
    fireEvent.change(screen.getByLabelText(/^Имя/), { target: { value: "Анна" } });
    fireEvent.change(screen.getByLabelText(/^Телефон или почта/), { target: { value: "anna@example.com" } });
    fireEvent.change(screen.getByLabelText(/^Тип проекта/), { target: { value: "Квартира" } });
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(screen.getByRole("button", { name: "Отправить запрос" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledOnce());
    expect(await screen.findByText(/Повторная отправка будет доступна через 60:00/)).toBeTruthy();
  });

  it("shows success and failure states", async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(new Response(null, { status: 201 })).mockResolvedValueOnce(new Response(null, { status: 500 }));
    vi.stubGlobal("fetch", fetchMock);
    render(<ContactForm />);
    fireEvent.click(screen.getByRole("button", { name: "Обсудить проект" }));
    fireEvent.change(screen.getByLabelText(/^Имя/), { target: { value: "Анна" } });
    fireEvent.change(screen.getByLabelText(/^Телефон или почта/), { target: { value: "anna@example.com" } });
    fireEvent.change(screen.getByLabelText(/^Тип проекта/), { target: { value: "Квартира" } });
    fireEvent.change(screen.getByLabelText("О проекте"), { target: { value: "Нужен проект квартиры" } });
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(screen.getByRole("button", { name: "Отправить запрос" }));
    expect(await screen.findByText(/Спасибо, заявка принята/)).toBeTruthy();
  });

  it("handles failed requests and closes the overlay", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("offline")));
    render(<ContactForm />);
    fireEvent.click(screen.getByRole("button", { name: "Обсудить проект" }));
    fireEvent.change(screen.getByLabelText(/^Имя/), { target: { value: "Анна" } });
    fireEvent.change(screen.getByLabelText(/^Телефон или почта/), { target: { value: "anna@example.com" } });
    fireEvent.change(screen.getByLabelText(/^Тип проекта/), { target: { value: "Квартира" } });
    fireEvent.change(screen.getByLabelText("О проекте"), { target: { value: "Нужен проект квартиры" } });
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(screen.getByRole("button", { name: "Отправить запрос" }));
    expect(await screen.findByText(/Не удалось отправить форму/)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Закрыть форму" }));
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
