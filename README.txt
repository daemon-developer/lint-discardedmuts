# Go Custom Linter: Discarded Modifications

This repository contains a custom Go linter that detects discarded modifications in your Go code. The linter helps identify places where modifications to variables or struct fields are made but not used, potentially indicating unnecessary computations or bugs.

## Installation

To install the custom linter, use the following command:

```bash
go install github.com/daemon-developer/lint-discardedmuts/cmd/discardedmuts@latest
```


## Usage

### Command Line

To use the linter from the command line, run the following command:

```bash
go vet -vettool=$(which discardedmuts) ./...
```

This will run the linter on all packages in the current directory and its subdirectories.

```bash
go vet -vettool=$(which discardedmuts) ./path/to/package
```

or

```bash
go vet -vettool=$(which discardedmuts) ./path/to/file.go
```

## Editor Integration

### Visual Studio Code Integration

To integrate the linter with Visual Studio Code, follow these steps:

1. Install the Go extension for Visual Studio Code.
2. Install the custom linter globally using the installation command mentioned above.
3. Install `golangci-lint` by running:
   ```bash
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   ```
4. Configure Visual Studio Code to use `golangci-lint` and the custom linter by adding the following settings to your `settings.json` file:
   ```json
   {
     "go.lintTool": "golangci-lint",
     "go.lintFlags": [
       "--enable-all",
       "--disable=lll",
       "--disable=gochecknoglobals",
       // ... other flags ...
       "--enable=wastedassign",
       "--out-format=colored-line-number",
       "--line-length=120",
       "--uniq-by-line=false",
       "--path-prefix=.",
       "--vettool=$(which discardedmuts)"
     ]
   }
   ```

With these settings in place, Visual Studio Code will automatically run the linter on your Go code whenever you save a file or manually trigger the linting process.

## Contributing

Contributions to this project are welcome! If you find any issues or have suggestions for improvements, please open an issue or submit a pull request.

## License

This project is licensed under the [MIT License](LICENSE).
