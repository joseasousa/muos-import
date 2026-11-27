# SaveSync for muOS and KNULLI

SaveSync is a prototype for syncing RetroArch saves on the RG40XX V. It uses a separate boot card for each firmware and a shared exFAT SD2 card. The interface shows the sync result and records the details in `sync.log`.

## Build and configure

1. Build the packages with Go 1.23 or later, a Linux ARM64 C compiler, and SDL2 development files for that target. On ARM64 Linux with `pkg-config`, the headers, and the development library installed, run `./package.sh`. To cross-compile, set `CC` to the ARM64 compiler and configure `PKG_CONFIG_SYSROOT_DIR` and `PKG_CONFIG_LIBDIR` so `pkg-config` can find SDL2 in the ARM64 sysroot. Use a sysroot compatible with both firmwares. The binary uses the SDL2 library provided by the firmware at runtime.
2. Copy `dist/muos/MUOS/application/SaveSync` to `MUOS/application/` on the muOS storage. On the KNULLI storage, copy the `SaveSync` folder and `SaveSync.sh` from `dist/knulli/roms/ports/` to `roms/ports/`. Use a test SD2 card and create an empty file named `.savesync-card` at its root.
3. Launch SaveSync from the firmware menu. Use the D-pad to navigate, **A** to confirm, and **B** to go back. On first launch, select the SD2 root, the ROM library, and the folders where RetroArch writes saves for the cores you use. SaveSync expects each save directly in the configured core folder, with the same base name as the ROM or ZIP file. Use Gambatte for GB and GBC, and mGBA for GBA. Select **Save configuration**, confirm with **Save**, and then select **Check configuration**. To change the settings later, select **Configure**.
4. Close RetroArch and select **Sync**. Check the result on screen and the details in `sync.log` next to `config.json`. Select **Exit** to return to the firmware menu. Power off the device before swapping the SD1 boot card and run SaveSync on the other firmware before playing.

To configure SaveSync from a file, copy `config.example.json` to `config.json` next to the executable and fill in the absolute paths from your device. Run `./savesync -config config.json -action check` to validate the configuration. For CLI development, use `go build .` and `go test ./...`.

## Backups and conflicts

SaveSync stores its data under `SaveSync/` on the SD2 card. The manifest records where each file came from and when it was modified. When both sides change a save, the most recent change wins. Ties require a manual choice. Keep the device clock accurate. From a terminal on the device, run the following commands to list backups and conflicts or restore a backup.

```sh
./savesync -config config.json -action list
./savesync -config config.json -action restore -key 'KEY' -hash HASH -confirm
```

## Save states and device validation

Keep save state syncing disabled until you validate both builds with disposable data. For testing, set `States` for the core, use the same identifier for the RetroArch and core combination in `StateProfile` on both sides, and set `BuildFiles` to the paths and SHA-256 hashes of the local executables and cores. Add the hashes of existing save states whose origin you have verified to `ApprovedStates`. SaveSync recognizes `.state`, `.state.auto`, and slots 1 through 999. If RetroArch loads the automatic slot at startup, the game may resume directly from that state.

Use the CLI to list backups and conflicts and to restore backups. On the RG40XX V, verify that the interface launches, SDL2 renders correctly, buttons work, the configured paths are correct, and exiting returns to the firmware menu. Also verify the builds, save state loading, and recovery after an interrupted write on the exFAT SD2 card.
