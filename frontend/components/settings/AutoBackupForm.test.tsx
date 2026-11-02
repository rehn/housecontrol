import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AutoBackupView } from "@/lib/api";
import { I18nProvider } from "@/lib/i18n";
import { AutoBackupForm } from "./AutoBackupForm";

afterEach(cleanup);

const view = (over: Partial<AutoBackupView> = {}): AutoBackupView => ({
  enabled: true,
  time: "03:00",
  keep: 14,
  available: true,
  dir: "/backups",
  encrypted: true,
  next: "2026-10-31T03:00:00+01:00",
  backups: [
    { name: "housecontrol-20261030T020000Z.hcbackup", size: 2048, created: "2026-10-30T02:00:00Z" },
    { name: "housecontrol-20261029T020000Z.hcbackup", size: 1900, created: "2026-10-29T02:00:00Z" },
  ],
  ...over,
});

function setup(v: AutoBackupView = view(), over: Record<string, unknown> = {}) {
  const props = {
    load: vi.fn(() => Promise.resolve(v)),
    save: vi.fn((s: { enabled: boolean; time: string; keep: number }) => Promise.resolve({ ...v, ...s })),
    run: vi.fn(() =>
      Promise.resolve({
        ...v,
        backups: [{ name: "housecontrol-20261030T140000Z.hcbackup", size: 2100, created: "2026-10-30T14:00:00Z" }, ...v.backups],
      }),
    ),
    download: vi.fn((name: string) => Promise.resolve({ blob: new Blob(["x"]), filename: name })),
    restore: vi.fn((_name: string) => Promise.resolve({ exported: "2026-10-30T03:00:00+01:00" })),
    saveFile: vi.fn(),
    onRestored: vi.fn(),
    reloadAfterMs: 0,
    ...over,
  };
  render(<AutoBackupForm {...props} />);
  return props;
}

const box = () => screen.getByRole("group", { name: "Automatic backups" });

describe("AutoBackupForm", () => {
  it("explains how to switch it on when there is no backup folder", async () => {
    setup(view({ available: false, dir: undefined, backups: [] }));
    expect(await within(box()).findByText(/BACKUP_DIR/)).toBeTruthy();
    expect(within(box()).queryByRole("button", { name: "Back up now" })).toBeNull();
  });

  it("shows the schedule, the next run and the stored backups", async () => {
    setup();
    const b = box();
    expect(await within(b).findByText(/Next backup/)).toBeTruthy();
    expect((within(b).getByLabelText("Time") as HTMLInputElement).value).toBe("03:00");
    expect((within(b).getByLabelText("Keep the newest") as HTMLInputElement).value).toBe("14");
    expect(within(b).getAllByRole("listitem")).toHaveLength(2);
    expect(within(b).getByText(/encrypted/i)).toBeTruthy();
  });

  it("saves at once when switched off or the time changes", async () => {
    const { save } = setup();
    const b = box();
    const on = (await within(b).findByRole("checkbox", { name: /every day/ })) as HTMLInputElement;
    fireEvent.click(on);
    expect(save).toHaveBeenLastCalledWith({ enabled: false, time: "03:00", keep: 14 });
    fireEvent.change(within(b).getByLabelText("Time"), { target: { value: "04:30" } });
    expect(save).toHaveBeenLastCalledWith({ enabled: false, time: "04:30", keep: 14 });
  });

  it("saves how many to keep only when done typing (it deletes old backups)", async () => {
    const { save } = setup();
    const keep = (await within(box()).findByLabelText("Keep the newest")) as HTMLInputElement;
    fireEvent.change(keep, { target: { value: "1" } });
    fireEvent.change(keep, { target: { value: "10" } });
    expect(save).not.toHaveBeenCalled();
    fireEvent.blur(keep);
    expect(save).toHaveBeenCalledTimes(1);
    expect(save).toHaveBeenLastCalledWith({ enabled: true, time: "03:00", keep: 10 });
  });

  it("refuses a keep outside 1-365", async () => {
    const { save } = setup();
    const keep = (await within(box()).findByLabelText("Keep the newest")) as HTMLInputElement;
    fireEvent.change(keep, { target: { value: "0" } });
    fireEvent.blur(keep);
    expect(save).not.toHaveBeenCalled();
    expect(within(box()).getByText(/1–365/)).toBeTruthy();
  });

  it("can back up now", async () => {
    const { run } = setup();
    fireEvent.click(await within(box()).findByRole("button", { name: "Back up now" }));
    expect(run).toHaveBeenCalled();
    await vi.waitFor(() => expect(within(box()).getAllByRole("listitem")).toHaveLength(3));
  });

  it("downloads a stored backup", async () => {
    const { download, saveFile } = setup();
    const first = (await within(box()).findAllByRole("listitem"))[0];
    fireEvent.click(within(first).getByRole("button", { name: "Download" }));
    expect(download).toHaveBeenCalledWith("housecontrol-20261030T020000Z.hcbackup");
    await vi.waitFor(() => expect(saveFile).toHaveBeenCalledWith(expect.any(Blob), "housecontrol-20261030T020000Z.hcbackup"));
  });

  it("restores a stored backup after a second tap", async () => {
    const { restore, onRestored } = setup();
    const second = (await within(box()).findAllByRole("listitem"))[1];
    fireEvent.click(within(second).getByRole("button", { name: "Restore" }));
    expect(restore).not.toHaveBeenCalled();
    fireEvent.click(within(second).getByRole("button", { name: "Tap again to replace all settings" }));
    expect(restore).toHaveBeenCalledWith("housecontrol-20261029T020000Z.hcbackup");
    expect(await within(box()).findByText(/Restored the backup/)).toBeTruthy();
    await vi.waitFor(() => expect(onRestored).toHaveBeenCalled());
  });

  it("shows when the last automatic backup failed", async () => {
    setup(view({ lastError: "disk full" }));
    expect(await within(box()).findByText(/disk full/)).toBeTruthy();
  });

  it("warns when the backups are not encrypted", async () => {
    setup(view({ encrypted: false }));
    expect(await within(box()).findByText(/BACKUP_PASSWORD/)).toBeTruthy();
  });

  it("is in Swedish", async () => {
    const props = {
      load: () => Promise.resolve(view()),
      save: vi.fn(),
      run: vi.fn(),
      download: vi.fn(),
      restore: vi.fn(),
    };
    render(
      <I18nProvider language="sv">
        <AutoBackupForm {...(props as unknown as Parameters<typeof AutoBackupForm>[0])} />
      </I18nProvider>,
    );
    expect(await screen.findByRole("button", { name: "Säkerhetskopiera nu" })).toBeTruthy();
  });
});
