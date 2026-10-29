import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { CalendarInput, CalendarSetting } from "@/lib/api";
import { I18nProvider } from "@/lib/i18n";
import { CalendarForm, PALETTE } from "./CalendarForm";

afterEach(cleanup);

const existing: CalendarSetting[] = [
  { id: "cal-1", name: "Familjen", color: "#f5c242", urlHint: "calendar.google.com/…/basic.ics" },
];

function setup(initial = existing, testResult = { ok: true, events: 12 }) {
  const onSave = vi.fn<(c: CalendarInput[]) => Promise<CalendarSetting[]>>((c) =>
    Promise.resolve(c.map((x, i) => ({ id: x.id ?? `new-${i}`, name: x.name, color: x.color, urlHint: "x" }))),
  );
  const test = vi.fn<(url: string) => Promise<{ ok: boolean; events?: number; error?: string }>>(() =>
    Promise.resolve(testResult),
  );
  render(<CalendarForm initial={initial} onSave={onSave} test={test} />);
  return { onSave, test };
}

describe("CalendarForm", () => {
  it("shows saved calendars with their link hint, never the link", () => {
    setup();
    expect((screen.getByDisplayValue("Familjen") as HTMLInputElement).value).toBe("Familjen");
    expect(screen.getByText("calendar.google.com/…/basic.ics")).toBeTruthy();
  });

  it("tests and adds a new calendar, then saves the whole list", async () => {
    const { onSave, test } = setup();
    const add = screen.getByRole("group", { name: "Add calendar" });
    fireEvent.change(within(add).getByLabelText("Name"), { target: { value: "Innebandy" } });
    fireEvent.change(within(add).getByLabelText("iCal link"), { target: { value: " webcal://example.com/team.ics " } });
    fireEvent.click(within(add).getByRole("radio", { name: PALETTE[1] }));

    fireEvent.click(within(add).getByRole("button", { name: "Test link" }));
    expect(await screen.findByText("Works – 12 events found.")).toBeTruthy();
    expect(test).toHaveBeenCalledWith("webcal://example.com/team.ics");

    // adding saves right away – no separate Save needed
    fireEvent.click(within(add).getByRole("button", { name: "Add calendar" }));
    await vi.waitFor(() => expect(onSave).toHaveBeenCalled());
    expect(onSave.mock.calls[0][0]).toEqual([
      { id: "cal-1", name: "Familjen", color: "#f5c242" }, // no url: the server keeps it
      { name: "Innebandy", color: PALETTE[1], url: "webcal://example.com/team.ics" },
    ]);
    expect(await screen.findByText("Calendar added.")).toBeTruthy();
    expect(screen.getByDisplayValue("Innebandy")).toBeTruthy(); // now in the list
    expect((within(add).getByLabelText("iCal link") as HTMLInputElement).value).toBe(""); // form cleared
  });

  it("keeps what was typed and says why when adding fails", async () => {
    const onSave = vi.fn(() => Promise.reject(new Error("boom")));
    render(<CalendarForm initial={[]} onSave={onSave} test={() => Promise.resolve({ ok: true })} />);
    const add = screen.getByRole("group", { name: "Add calendar" });
    fireEvent.change(within(add).getByLabelText("Name"), { target: { value: "X" } });
    fireEvent.change(within(add).getByLabelText("iCal link"), { target: { value: "https://x/y.ics" } });
    fireEvent.click(within(add).getByRole("button", { name: "Add calendar" }));
    expect(await screen.findByText(/Could not save: boom/)).toBeTruthy();
    expect((within(add).getByLabelText("iCal link") as HTMLInputElement).value).toBe("https://x/y.ics");
    expect(screen.getAllByDisplayValue("X")).toHaveLength(1); // still only in the add form
  });

  it("shows why a link doesn't work", async () => {
    setup([], { ok: false, error: "not an iCal calendar (check the link)" } as never);
    const add = screen.getByRole("group", { name: "Add calendar" });
    fireEvent.change(within(add).getByLabelText("iCal link"), { target: { value: "https://example.com/" } });
    fireEvent.click(within(add).getByRole("button", { name: "Test link" }));
    expect(await screen.findByText("Doesn't work: not an iCal calendar (check the link)")).toBeTruthy();
  });

  it("says what is missing instead of doing nothing", () => {
    const { onSave } = setup([]);
    const add = screen.getByRole("group", { name: "Add calendar" });
    const btn = within(add).getByRole("button", { name: "Add calendar" }) as HTMLButtonElement;
    expect(btn.disabled).toBe(false);
    fireEvent.click(btn);
    expect(screen.getByText("Enter a name and an iCal link first.")).toBeTruthy();
    fireEvent.change(within(add).getByLabelText("iCal link"), { target: { value: "https://x/y.ics" } });
    fireEvent.click(btn);
    expect(screen.getByText("Enter a name and an iCal link first.")).toBeTruthy();
    expect(onSave).not.toHaveBeenCalled();
  });

  const two: CalendarSetting[] = [...existing, { id: "cal-2", name: "Jobb", color: "#4fa8e8", urlHint: "y" }];

  it("has no Save button – every change is saved on its own", () => {
    setup(two);
    expect(screen.queryByRole("button", { name: "Save" })).toBeNull();
  });

  it("saves a new name when leaving the field", async () => {
    const { onSave } = setup(two);
    const input = screen.getByDisplayValue("Familjen");
    fireEvent.change(input, { target: { value: "Hemma" } });
    expect(onSave).not.toHaveBeenCalled(); // not on every keystroke
    fireEvent.blur(input);
    await vi.waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(onSave.mock.calls[0][0]).toEqual([
      { id: "cal-1", name: "Hemma", color: "#f5c242" },
      { id: "cal-2", name: "Jobb", color: "#4fa8e8" },
    ]);
    expect(await screen.findByText("Saved.")).toBeTruthy();
  });

  it("saves the name on Enter, and not at all when nothing changed", async () => {
    const { onSave } = setup(two);
    const input = screen.getByDisplayValue("Jobb");
    fireEvent.blur(input);
    expect(onSave).not.toHaveBeenCalled();
    fireEvent.change(input, { target: { value: "Arbete" } });
    fireEvent.keyDown(input, { key: "Enter" });
    await vi.waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(onSave.mock.calls[0][0][1]).toEqual({ id: "cal-2", name: "Arbete", color: "#4fa8e8" });
  });

  it("puts the old name back if the field is left empty", () => {
    const { onSave } = setup(two);
    const input = screen.getByDisplayValue("Jobb");
    fireEvent.change(input, { target: { value: "  " } });
    fireEvent.blur(input);
    expect(onSave).not.toHaveBeenCalled();
    expect(screen.getByDisplayValue("Jobb")).toBeTruthy();
  });

  it("saves a new colour at once", async () => {
    const { onSave } = setup(two);
    const row = screen.getAllByRole("listitem")[1];
    fireEvent.click(within(row).getByRole("radio", { name: PALETTE[4] }));
    await vi.waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(onSave.mock.calls[0][0][1]).toEqual({ id: "cal-2", name: "Jobb", color: PALETTE[4] });
  });

  it("saves at once when a calendar is removed", async () => {
    const { onSave } = setup(two);
    fireEvent.click(screen.getByRole("button", { name: "Remove Jobb" }));
    await vi.waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(onSave.mock.calls[0][0]).toEqual([{ id: "cal-1", name: "Familjen", color: "#f5c242" }]);
    expect(screen.queryByDisplayValue("Jobb")).toBeNull();
  });

  it("shows when a change could not be saved", async () => {
    const onSave = vi.fn(() => Promise.reject(new Error("boom")));
    render(<CalendarForm initial={two} onSave={onSave} test={() => Promise.resolve({ ok: true })} />);
    fireEvent.click(screen.getByRole("button", { name: "Remove Jobb" }));
    expect(await screen.findByText(/Could not save: boom/)).toBeTruthy();
  });

  it("is in Swedish when Swedish is chosen", () => {
    render(
      <I18nProvider language="sv">
        <CalendarForm initial={[]} onSave={() => Promise.resolve([])} test={() => Promise.resolve({ ok: true })} />
      </I18nProvider>,
    );
    const add = screen.getByRole("group", { name: "Lägg till kalender" });
    expect(within(add).getByLabelText("iCal-länk")).toBeTruthy();
    expect(within(add).getByRole("button", { name: "Testa länken" })).toBeTruthy();
    expect(screen.getByText("Inga kalendrar ännu.")).toBeTruthy();
  });
});
