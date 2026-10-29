# housecontrol – User guide

housecontrol is a small home control panel for Zigbee lights and outlets
(through zigbee2mqtt). It is made for wall-mounted or hand-held tablets, but
works in any browser. This guide explains how to use it. Installing and
running the app is described in the [README](../README.md).

The app shows English or Swedish texts (see [Language](#language)); this guide
uses the English names of buttons and menus.

## Contents

- [What everyone can do – and what needs a login](#what-everyone-can-do--and-what-needs-a-login)
- [The home page](#the-home-page)
- [Switching lights and outlets](#switching-lights-and-outlets)
- [Cards](#cards)
- [Logging in and out](#logging-in-and-out)
- [Editing the home page](#editing-the-home-page)
- [Tabs](#tabs)
- [Grafana](#grafana)
- [Schedules](#schedules)
- [Settings](#settings) (including [Backup](#backup))
- [New lights from zigbee2mqtt](#new-lights-from-zigbee2mqtt)
- [Tips for a wall tablet](#tips-for-a-wall-tablet)
- [Troubleshooting](#troubleshooting)

---

## What everyone can do – and what needs a login

| Without logging in | Logged in (admin) |
|---|---|
| See the home page and all tabs | Everything on the left, plus: |
| Switch lights, outlets and groups on/off | Edit the home page (cards, columns, tabs) |
| Change brightness and warmth | All settings, including schedules |
| Use the weather and calendar cards | |

If the server has no admin password set, everything is open and
**Settings → Account** shows a warning.

## The home page

At the top:

- **Title** – your own name for the app (set under **Settings → General**).
- **Status badge** – the connection to the lights:
  - `live` – all good,
  - `connecting` – connecting to the server or the MQTT broker,
  - `offline` – the browser has lost contact with the server (it reconnects by itself),
  - `not configured` / `error` – the MQTT broker isn't set up or can't be reached
    (a banner below explains more).
- **Edit** and **Settings** – only when logged in.
- **Log in / Log out** – the door icon (only when a password is set).

Under the title there is a row of **tabs** when you have more than one
(see [Tabs](#tabs)). The rest of the page is your **cards** in columns.

## Switching lights and outlets

- **Tap** the big two-part button on a card: the lit half shows the current
  state. Tap to switch.
- **Press and hold** a card (about half a second), or right-click it with a
  mouse, to open its **control panel**:
  - on/off,
  - **Brightness** slider (dimmable lights),
  - **Warmth** slider, from cold to warm white (lights that support it).

  Close it with **Close**, by tapping outside it, or with Esc.
- A **group card** switches several lights at once. It shows how many are on
  (e.g. "1 of 2 on"). One tap turns them **all off** if any is on, otherwise
  **all on**. Press and hold to set the brightness of all of them.
- Cards of devices that are **offline** are dimmed and can't be switched.
- Outlets that measure power show the current use in watts.

Changes made anywhere – on another tablet, in zigbee2mqtt, with a wall switch
or by a schedule – show up on every screen right away.

## Cards

| Card | Shows |
|---|---|
| **Device** | One light or outlet. You can give it its own name. |
| **Group** | Several lights/outlets switched together. |
| **Clock** | Time and date. |
| **Weather** | Weather now, today's high/low, today's sunrise and sunset, and the next days, for the place chosen under Settings → Weather. |
| **Calendar** | Events from your iCal calendars, a week at a time. |
| **Grafana panel** | One graph or number from Grafana (see [Grafana](#grafana)). |

### The calendar card

- Shows **7 days**, one section per day; days without events are left out.
  All-day events come first, then by time. Each calendar has its own colour.
- **‹** and **›** step a week back or forward; the dates shown are at the top.
  **Today** jumps back.
- After 5 minutes on another week, the card goes back to today by itself.
- It moves on to the new day just after midnight, and reloads when the
  tablet's screen comes back on, so it always shows the right day in the
  morning.
- If one of several calendars can't be loaded, the card says so and shows the
  rest.

## Logging in and out

1. Tap the **door icon** at the top right.
2. Enter the user name (usually `admin`) and the password, and tap **Log in**.

You stay logged in on that device for 30 days, or until you tap the door icon
again (or **Settings → Account → Log out**). After five wrong passwords, that
device has to wait five minutes before trying again.

## Editing the home page

Log in and tap **Edit**. A toolbar appears:

- **+ Add card** – choose a kind of card:
  - **Device**: pick the light/outlet; the name is optional (defaults to the
    device's name). Devices that already have a card are marked "on dashboard".
  - **Group**: give it a name and tick the lights/outlets.
  - **Clock**, **Weather**, **Calendar**: added at once.
  - **Grafana panel**: see [Grafana](#grafana).
- **− / +** next to **Columns** – 1 to 6 columns. Cards in removed columns
  move to the last column.
- **Reset** – start over with a clock and every light/outlet.
- **Move a card**: press and hold it briefly, then drag it to its new place
  (with a mouse: just drag). Drop it on an empty column to put it there.
- **Pencil** on a card – change it (device, group and Grafana cards).
- **×** on a card – remove it.
- **Save** stores the layout for **all screens**; **Cancel** throws your
  changes away.

While editing, the switches on the cards don't react, so you can't switch
something by mistake.

## Tabs

Tabs let you have several pages, e.g. "Home", "Upstairs", "Garden" or a
Grafana dashboard.

- **Add a tab**: log in → **Edit** → **+ Add tab** next to the tabs. Give it
  a name and choose:
  - **Page of cards** – a new page you fill with cards like the home page,
  - **Grafana dashboard** – a whole Grafana dashboard (see [Grafana](#grafana)).
- **Rename, move or remove** the open tab: in edit mode, tap the **pencil**
  next to the tabs. **Move left / Move right** change the order. **Remove tab**
  needs two taps ("Tap again to remove"), so a page of cards isn't lost by
  mistake. The last tab can't be removed.
- Tab changes are saved at once; the cards on a page are saved with **Save**.
- The tab row is hidden while there is only one tab.
- The open tab is part of the address (e.g. `…/#garden`), so a bookmark or
  reload opens the same tab.

Your first page is called **Home** unless you give it another name.

## Grafana

You can show Grafana in two ways:

**A whole dashboard as a tab** – add a tab of kind **Grafana dashboard** and
paste the dashboard's address from Grafana (the one in the browser's address
bar). It fills the tab without Grafana's menus. **Open in Grafana** below it
opens the same dashboard in Grafana itself.

**One panel as a card** – **Edit → + Add card → Grafana panel**:

1. In Grafana, open the panel's menu → **Share** → **Embed**, and copy the link
   or the whole `<iframe …>` code. (A link to a panel opened with **View**
   works too; the app turns it into an embed link.)
2. Paste it into **Panel link**.
3. Choose the **Height** in pixels (120–1200) and, if you like, a name.

Grafana follows the app's light/dark look unless the link says otherwise.

If a Grafana tab or card stays empty or says it refused to connect, Grafana
isn't set up to be shown inside other pages yet, or this browser isn't logged
in to Grafana. Log in to Grafana once in the same browser; if that doesn't
help, the person who runs Grafana can follow the setup in the README.

## Schedules

**Settings → Schedules** turns lights and outlets on and off by themselves.
The server runs them, so no screen needs to be open.

**Add a schedule** with **+ Add schedule**:

1. **Name**, e.g. "Garden lights".
2. Tick the **lights and outlets** it should switch.
3. **Days** – tap the weekdays it should run. None chosen = every day.
4. **Turn on** and/or **Turn off** – tick the ones you want, and choose:
   - **At a time** – e.g. 06:30,
   - **Sunrise** or **Sunset** – optionally a number of **Minutes before/after**
     it, and **Not before** / **Not after** a time. Example: "30 minutes
     before sunset, not before 16:00" lights up early on winter afternoons
     but never before four.
5. **Set brightness** (optional) – the brightness the lights get when the
   schedule turns them on. Outlets and non-dimmable lights are just switched on.
6. **Add**.

The list shows each schedule in words, e.g. *"On 30 min before sunset, not
before 16:00 (40 %) · Off 23:30 · Weekdays"*, and when it runs next
(*"turns on today 16:12"*).

- The **switch** at the left of a schedule pauses or resumes it.
- The **pencil** changes it; **Remove schedule** (two taps) deletes it.
- Today's sunrise and sunset for your place are shown at the top.

Good to know:

- Sunrise and sunset are calculated for the place chosen under
  **Settings → Weather**. Without a place, the sun parts of schedules don't run
  (a warning says so); fixed times still do.
- Times follow summer/winter time automatically. During midnight sun or polar
  night, sun-based switching is skipped on days without a sunrise/sunset.
- Schedules switch within about 15 seconds of the time.
- If the app was down at the time of an event, the event is skipped – it isn't
  run late.
- A schedule only switches at its times; you can still switch the lights by hand
  in between.

## Settings

Log in and tap **Settings**. The menu on the left (a row of tabs on a phone)
has these sections:

### General
**App title** – the name at the top of the home page and in the browser tab.
Leave it empty for the default.

### Appearance
**Light**, **Dark** or **Automatic** (follows the device's own setting). This
applies **only to the device you choose it on**, so the kitchen tablet can be
dark while the computer is light.

### Language
English or Swedish for every text the app shows, on **all screens**. Names you
have given things yourself are not translated.

### Weather
1. Choose a **weather service**: SMHI (Sweden and the Nordics), MET Norway/Yr
   or Open-Meteo (worldwide). All are free.
2. **Search** for your place and choose it in the list, or tap
   **Use my position**.
3. **Save**.

The place is also used for sunrise/sunset in [Schedules](#schedules).

### Calendars
Add calendars by pasting their **iCal link** (`.ics` or `webcal://`), e.g.
Google Calendar's "Secret address in iCal format", iCloud or Outlook:

1. Enter a **Name**, paste the **iCal link** and pick a **colour**.
2. **Test link** checks it and says how many events it found.
3. **Add calendar** – saved at once.

Rename, recolour or remove calendars in the list; changes are saved at once.
The link is kept on the server and never shown again. Calendars are refreshed
every 15 minutes.

### Schedules
See [Schedules](#schedules).

### MQTT broker
Where zigbee2mqtt's MQTT broker is, e.g. `192.168.1.201` or
`tcp://pi.lan:1883`, with user name and password if the broker needs them.

- **Test connection** tries the settings without changing anything.
- **Save & connect** saves and reconnects.
- The password field shows "(unchanged)" – leave it to keep the saved password.
- **zigbee2mqtt base topic** is usually `zigbee2mqtt`.

### Backup
**Save a backup** downloads everything you have set up – home page and tabs,
schedules, calendars, weather, language, title and the MQTT settings – as one
file.

- **Protect with a password** (recommended, on by default): choose a password
  (at least 8 characters, typed twice). The file
  (`housecontrol-backup-<date>.hcbackup`) is compressed and **encrypted** with
  it. Keep the password safe; it can't be recovered.
- Without a password you get a readable `housecontrol-backup-<date>.json` –
  handy for looking inside, but anyone who gets it can read the MQTT password
  and your calendars' secret links. Keep it somewhere safe.

**Restore a backup** – choose the file (`.hcbackup` or `.json`), enter its
password if it is protected, and tap **Restore**, then once more to confirm.
All settings are replaced with the backup's and used at once, and the page
reloads. If the password is wrong or the file isn't a valid backup, nothing is
changed. The admin password is not part of backups.

Good moments for a backup: before upgrading, before moving the app to another
server, and after setting up something you'd hate to redo.

### Account
Who is logged in, and **Log out**.

## New lights from zigbee2mqtt

- When you pair a new light or outlet in zigbee2mqtt it appears in the app by
  itself; nothing needs restarting.
- If you have **never saved a layout**, the home page shows every light and
  outlet, including the new one.
- If you **have** saved a layout, add it yourself: **Edit → + Add card →
  Device** (or add it to a group or schedule).
- **Renaming** a device in zigbee2mqtt keeps its cards and schedules (they
  follow the device, not its name).
- If a device is **removed** from zigbee2mqtt, its card shows "Device not
  found" in edit mode – remove the card there, and untick it in schedules.

## Tips for a wall tablet

- Open the app in full screen, or use the browser's **Add to Home screen** so it
  opens like an app without the address bar.
- Choose **Automatic** under **Settings → Appearance** to follow the tablet's
  own day/night setting, or **Dark** for a kitchen at night.
- The tablet can stay logged out: everyone can still switch lights, while only
  logged-in users can change the layout and settings.
- The calendar card goes back to today by itself and shows the new day in the
  morning.

## Troubleshooting

| Problem | What to do |
|---|---|
| Status says `offline` | The tablet can't reach the server. Check the network; the app reconnects by itself. |
| Banner about the MQTT broker | Check **Settings → MQTT broker** and **Test connection**. |
| "Connected, but no devices yet" | zigbee2mqtt hasn't sent its device list; check the **base topic** under Settings → MQTT broker. |
| A card is dimmed | The device is offline in zigbee2mqtt (no power, out of range). |
| A light doesn't react | Check that it works in zigbee2mqtt; check the status badge. |
| Weather card asks for a place | Choose one under **Settings → Weather**. |
| Calendar says some calendars failed | Test the link under **Settings → Calendars**; the calendar may have been moved or made private. |
| Grafana is empty / "refused to connect" | Log in to Grafana in the same browser; otherwise see the README's Grafana setup. |
| A schedule didn't run | Is it switched on? Are today's weekday and the times right? Sun-based? Then a place must be chosen under Weather. Was the app running at that time? |
| "Too many wrong attempts" when logging in | Wait five minutes and try again. |
