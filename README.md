# Pharos

## Package and file structure

- `cmd`: Cobra commands
- `controllers`: API controllers, if possible per database model
- `globals`: Put any global variables here.
- `integrations`: Sources, Flows and Sinks for go-streams, can also contain functions.
- `logging`: Some logging that we can later change if needed.
- `models`: Database models and non-database models, such as config. Try not to put functionality here, except where it makes sense.
- `routing`: Flows definition for go-streams
- `.vscode`: How to launch and debug parts of the application with Visual Studio Code. Do not put any secrets here.