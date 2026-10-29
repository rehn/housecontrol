import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { I18nProvider } from "@/lib/i18n";
import { appTitle, DEFAULT_TITLE } from "@/lib/title";
import { TitleForm } from "./TitleForm";

afterEach(cleanup);

const saveMock = () => vi.fn<(title: string) => Promise<void>>(() => Promise.resolve());

describe("appTitle", () => {
  it("uses the default when no title is set", () => {
    expect(appTitle(undefined)).toBe(DEFAULT_TITLE);
    expect(appTitle("   ")).toBe("housecontrol");
  });

  it("uses the custom title", () => {
    expect(appTitle(" Stugan ")).toBe("Stugan");
  });
});

describe("TitleForm", () => {
  it("saves the typed title", async () => {
    const onSave = saveMock();
    render(<TitleForm value="" onSave={onSave} />);
    fireEvent.change(screen.getByLabelText("App title"), { target: { value: "  Stugan " } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await vi.waitFor(() => expect(onSave).toHaveBeenCalledWith("Stugan"));
    expect(await screen.findByText("Saved.")).toBeTruthy();
  });

  it("shows the default as placeholder and allows clearing back to it", async () => {
    const onSave = saveMock();
    render(<TitleForm value="Stugan" onSave={onSave} />);
    const input = screen.getByLabelText("App title") as HTMLInputElement;
    expect(input.value).toBe("Stugan");
    expect(input.placeholder).toBe("housecontrol");
    fireEvent.change(input, { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await vi.waitFor(() => expect(onSave).toHaveBeenCalledWith(""));
  });

  it("only enables Save when the title changed", () => {
    render(<TitleForm value="Stugan" onSave={saveMock()} />);
    const save = screen.getByRole("button", { name: "Save" }) as HTMLButtonElement;
    expect(save.disabled).toBe(true);
    fireEvent.change(screen.getByLabelText("App title"), { target: { value: "Hemma" } });
    expect(save.disabled).toBe(false);
  });

  it("limits the length to 60 characters", () => {
    render(<TitleForm value="" onSave={saveMock()} />);
    expect((screen.getByLabelText("App title") as HTMLInputElement).maxLength).toBe(60);
  });

  it("shows an error if saving fails, in the chosen language", async () => {
    const onSave = vi.fn<(title: string) => Promise<void>>(() =>
      Promise.reject(new ApiError("cannot reach the housecontrol backend", "backend_unreachable")),
    );
    render(
      <I18nProvider language="sv">
        <TitleForm value="" onSave={onSave} />
      </I18nProvider>,
    );
    fireEvent.change(screen.getByLabelText("Appens titel"), { target: { value: "Stugan" } });
    fireEvent.click(screen.getByRole("button", { name: "Spara" }));
    expect(
      await screen.findByText("Kunde inte spara titeln: Kan inte nå housecontrol-backenden"),
    ).toBeTruthy();
  });
});
