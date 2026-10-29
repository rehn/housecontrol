import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import { I18nProvider } from "@/lib/i18n";
import { BackupForm } from "./BackupForm";

afterEach(cleanup);

function setup(over: Partial<Parameters<typeof BackupForm>[0]> = {}) {
  const props = {
    download: vi.fn((_pw: string | null) => Promise.resolve({ blob: new Blob(["x"]), filename: "housecontrol-backup-2026-10-29.hcbackup" })),
    restore: vi.fn((_f: File, _pw: string) => Promise.resolve({ exported: "2026-10-29T10:00:00+01:00" })),
    save: vi.fn(),
    onRestored: vi.fn(),
    reloadAfterMs: 0,
    ...over,
  };
  render(<BackupForm {...props} />);
  return props;
}

const file = () => new File([new Uint8Array([1, 2, 3])], "housecontrol-backup-2026-10-29.hcbackup");

describe("BackupForm – save", () => {
  it("needs a password of 8+ characters, typed twice", () => {
    const { download } = setup();
    const box = screen.getByRole("group", { name: "Save a backup" });
    const btn = within(box).getByRole("button", { name: "Download backup" }) as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
    fireEvent.change(within(box).getByLabelText("Password"), { target: { value: "kort" } });
    expect(within(box).getByText("At least 8 characters.")).toBeTruthy();
    fireEvent.change(within(box).getByLabelText("Password"), { target: { value: "familjens-lösen" } });
    fireEvent.change(within(box).getByLabelText("Repeat password"), { target: { value: "familjens-lösem" } });
    expect(within(box).getByText("The passwords don't match.")).toBeTruthy();
    expect(btn.disabled).toBe(true);
    fireEvent.click(btn);
    expect(download).not.toHaveBeenCalled();
  });

  it("downloads the encrypted file", async () => {
    const { download, save } = setup();
    const box = screen.getByRole("group", { name: "Save a backup" });
    fireEvent.change(within(box).getByLabelText("Password"), { target: { value: "familjens-lösen" } });
    fireEvent.change(within(box).getByLabelText("Repeat password"), { target: { value: "familjens-lösen" } });
    fireEvent.click(within(box).getByRole("button", { name: "Download backup" }));
    expect(download).toHaveBeenCalledWith("familjens-lösen");
    expect(await within(box).findByText(/housecontrol-backup-2026-10-29\.hcbackup/)).toBeTruthy();
    expect(save).toHaveBeenCalledWith(expect.any(Blob), "housecontrol-backup-2026-10-29.hcbackup");
    // the password fields are cleared afterwards
    expect((within(box).getByLabelText("Password") as HTMLInputElement).value).toBe("");
  });
});

describe("BackupForm – save without a password", () => {
  it("can save a readable file, with a warning", async () => {
    const { download, save } = setup({
      download: vi.fn(() => Promise.resolve({ blob: new Blob(["{}"]), filename: "housecontrol-backup-2026-10-29.json" })),
    });
    const box = screen.getByRole("group", { name: "Save a backup" });
    const protect = within(box).getByRole("checkbox", { name: /Protect with a password/ }) as HTMLInputElement;
    expect(protect.checked).toBe(true);
    fireEvent.click(protect);
    expect(within(box).queryByLabelText("Password")).toBeNull();
    expect(within(box).getByText(/readable by anyone/)).toBeTruthy();
    fireEvent.click(within(box).getByRole("button", { name: "Download backup" }));
    expect(download).toHaveBeenCalledWith(null);
    await vi.waitFor(() => expect(save).toHaveBeenCalledWith(expect.any(Blob), "housecontrol-backup-2026-10-29.json"));
  });
});

describe("BackupForm – restore", () => {
  function choose(box: HTMLElement, pw = "familjens-lösen") {
    fireEvent.change(within(box).getByLabelText("Backup file"), { target: { files: [file()] } });
    fireEvent.change(within(box).getByLabelText(/^Password/), { target: { value: pw } });
  }

  it("needs a file, and asks before replacing everything", async () => {
    const { restore, onRestored } = setup();
    const box = screen.getByRole("group", { name: "Restore a backup" });
    const btn = within(box).getByRole("button", { name: "Restore" }) as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
    choose(box);
    fireEvent.click(btn);
    expect(restore).not.toHaveBeenCalled();
    fireEvent.click(within(box).getByRole("button", { name: "Tap again to replace all settings" }));
    expect(restore).toHaveBeenCalledWith(expect.any(File), "familjens-lösen");
    expect(await within(box).findByText(/Restored the backup from/)).toBeTruthy();
    await vi.waitFor(() => expect(onRestored).toHaveBeenCalled());
  });

  it("restores an unprotected file without a password", async () => {
    const { restore } = setup();
    const box = screen.getByRole("group", { name: "Restore a backup" });
    fireEvent.change(within(box).getByLabelText("Backup file"), {
      target: { files: [new File(["{}"], "housecontrol-backup-2026-10-29.json")] },
    });
    const btn = within(box).getByRole("button", { name: "Restore" }) as HTMLButtonElement;
    expect(btn.disabled).toBe(false);
    fireEvent.click(btn);
    fireEvent.click(within(box).getByRole("button", { name: "Tap again to replace all settings" }));
    expect(restore).toHaveBeenCalledWith(expect.any(File), "");
  });

  it("asks for the password when the file is protected", async () => {
    setup({ restore: vi.fn(() => Promise.reject(new ApiError("protected", "backup_needs_password", 400))) });
    const box = screen.getByRole("group", { name: "Restore a backup" });
    fireEvent.change(within(box).getByLabelText("Backup file"), { target: { files: [file()] } });
    fireEvent.click(within(box).getByRole("button", { name: "Restore" }));
    fireEvent.click(within(box).getByRole("button", { name: "Tap again to replace all settings" }));
    expect(await within(box).findByText(/protected with a password/)).toBeTruthy();
  });

  it("says when the password is wrong", async () => {
    setup({
      restore: vi.fn(() => Promise.reject(new ApiError("wrong", "backup_wrong_password", 400))),
    });
    const box = screen.getByRole("group", { name: "Restore a backup" });
    choose(box, "fel-lösenord");
    fireEvent.click(within(box).getByRole("button", { name: "Restore" }));
    fireEvent.click(within(box).getByRole("button", { name: "Tap again to replace all settings" }));
    expect(await within(box).findByText(/Wrong password, or the file is damaged/)).toBeTruthy();
  });

  it("is in Swedish", () => {
    render(
      <I18nProvider language="sv">
        <BackupForm download={vi.fn()} restore={vi.fn()} save={vi.fn()} onRestored={vi.fn()} />
      </I18nProvider>,
    );
    expect(screen.getByRole("group", { name: "Spara en säkerhetskopia" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Ladda ner säkerhetskopia" })).toBeTruthy();
    expect(screen.getByRole("group", { name: "Återställ en säkerhetskopia" })).toBeTruthy();
  });
});
