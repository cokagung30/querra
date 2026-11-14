# Querra Privacy Policy

*Last updated: November 14, 2026*

Querra is a desktop database client published by Bali Digital Karya. This policy explains what Querra does with your information. In short: everything stays on your computer, except what Querra sends to the database and SSH servers you choose to connect to. The developer receives nothing.

## What Querra stores on your computer

- **Connection settings** you enter: connection names, host names, ports, user names, database names, TLS and SSH options, and the file paths of any certificates or SSH keys. Querra stores the paths, not the files' contents.
- **Passwords and passphrases** for your connections, kept in your operating system's credential store (Windows Credential Manager or the macOS Keychain), never in Querra's own files.
- **App settings and layout**, such as panel sizes, preferences and keyboard shortcuts.
- **Query history and snippets**, when you use those features.
- **Log files** that help diagnose problems. Logs never contain passwords or data from your databases.
- **Connection export files**, when you export connections to share them or move them to another computer. They hold the connection settings above, never passwords or passphrases; importing one asks for the passwords again.

This information stays on your computer, in Querra's data folder, your system's credential store, and any export files you save. Querra never uploads it anywhere.

## What Querra sends over the network

- **To the database and SSH servers you configure:** your user name, password or key, and the queries you run, so Querra can connect and show you results. These servers are run by you or your organization, not by the developer.
- **Update checks:** on Windows, Querra is installed and updated through the Microsoft Store. On macOS, Querra can check for updates; that check sends only the app version and platform, and you can turn it off.

Querra has no telemetry, analytics, advertising, or crash reporting. Data you view or export from your databases stays on your computer, in the files you choose.

## Removing your data

Deleting a connection in Querra also deletes its saved password and passphrases from the credential store. Uninstalling Querra and deleting its data folder removes the rest.

## Third parties

The Microsoft Store (Windows) and GitHub (macOS downloads and update checks) are operated under their own privacy statements. Querra does not share your information with anyone else.

## Children

Querra is a tool for software developers and is not directed at children.

## Changes and contact

Changes to this policy are published in this file, with a new date above. Questions are welcome as an issue at <https://github.com/cokagung30/querra/issues>.
