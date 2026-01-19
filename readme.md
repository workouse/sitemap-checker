# Sitemap Checker

[![Go](https://github.com/workouse/sitemap-checker/actions/workflows/go.yml/badge.svg)](https://github.com/workouse/sitemap-checker/actions/workflows/go.yml)
[![Scrutinizer Code Quality](https://scrutinizer-ci.com/g/workouse/sitemap-checker/badges/quality-score.png?b=master)](https://scrutinizer-ci.com/g/workouse/sitemap-checker/?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/workouse/sitemap-checker)](https://goreportcard.com/report/github.com/workouse/sitemap-checker)

A simple but powerful command-line tool to validate URLs in a sitemap file. It checks for broken links and ensures all your sitemap entries are accessible.

![Sitemap Checker in Action](https://user-images.githubusercontent.com/803964/211917944-757cba14-5335-4989-923c-da3476c5cfbb.png)

## Features

- **Concurrent URL Validation**: Checks multiple URLs at once for faster processing.
- **Sitemap Index Support**: Can parse and validate sitemap index files.
- **Cross-Platform**: Builds for Linux, macOS, and Windows.
- **Lightweight**: A single binary with no external dependencies.

## Installation

You can download the latest pre-compiled binaries for your operating system from the [Releases](https://github.com/workouse/sitemap-checker/releases) page.

### Linux

```bash
# Download the latest release
wget https://github.com/workouse/sitemap-checker/releases/latest/download/sitemap-checker_amd64.deb

# Install the package
sudo dpkg -i sitemap-checker_amd64.deb

# Run the tool
sitemap-checker --help
```

### macOS

You can use Homebrew to install:

```bash
brew tap workouse/sitemap-checker
brew install sitemap-checker
```

Or, you can install manually:

```bash
# Download and unzip the latest release
wget https://github.com/workouse/sitemap-checker/releases/latest/download/sitemap-checker_darwin_amd64.zip
unzip sitemap-checker_darwin_amd64.zip

# Make the binary executable and move it to your PATH
chmod +x sitemap-checker
sudo mv sitemap-checker /usr/local/bin/
```

### Windows

1.  Download the `sitemap-checker_windows_amd64.zip` file from the [Releases](https://github.com/workouse/sitemap-checker/releases) page.
2.  Extract the `sitemap-checker.exe` file.
3.  Place it in a directory that is included in your system's `PATH`.

## Usage

### Validate a Single Sitemap

To validate a standard `sitemap.xml` file, use the `-uri` flag to specify the URL and the `-out` flag for the output file.

```bash
sitemap-checker -uri=http://example.com/sitemap.xml -out=validated-sitemap.xml
```

### Validate a Sitemap Index

If your sitemap is a sitemap index file, add the `-index` flag. The tool will fetch and validate all the sitemaps listed in the index.

```bash
sitemap-checker -uri=http://example.com/sitemap-index.xml -index
```

## Contributing

Contributions are welcome! If you find a bug or have a feature request, please [open an issue](https://github.com/workouse/sitemap-checker/issues). If you'd like to contribute code, please follow these steps:

1.  **Fork the repository** on GitHub.
2.  **Clone your fork** to your local machine.
3.  **Create a new branch** for your changes.
4.  **Make your changes** and commit them with a descriptive message.
5.  **Push your changes** to your fork.
6.  **Open a pull request** to the `master` branch of the original repository.

Please make sure your code adheres to the existing style and that all tests pass.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
