# Expirybot

Expirybot is a cross-platform command line application written in Go that checks domain reachability and SSL certificate validity. It reads a list of domain names from a file and for each one, it checks if the domain is reachable and if so, it checks the validity of that domain's SSL certificate. The app will then print to stdout if the duration of validity is within a specified threshold.

## Features

- Reads domain names from a configuration file
- Checks domain reachability via DNS lookup
- Validates SSL certificates for reachable domains
- Reports certificates expiring within a configurable threshold
- Follows XDG Base Directory Specification for configuration files
- Cross-platform support (Linux and macOS)

## Installation

### Building from Source

1. Ensure you have Go installed on your system (version 1.18 or later recommended)
2. Clone this repository or download the source code
3. Navigate to the project directory
4. Build the application:

```bash
go build -o expirybot
```

5. (Optional) Move the binary to a directory in your PATH:

```bash
# Linux/macOS
sudo mv expirybot /usr/local/bin/
```

## Usage

### Basic Usage

Run the application without any arguments to use the default configuration:

```bash
./expirybot
```

By default, the application will:
- Look for a configuration file at `~/.config/expirybot/expirybot.conf`
- Use a threshold of 14 days for certificate expiration warnings
- Only print domains with certificates expiring within the threshold

### Command Line Options

```
Usage of ./expirybot:
  -file string
        Path to domains file (overrides default config file)
  -add string
        Add a domain to check (format: domain.com[,threshold])
```

Examples:

```bash
# Use a custom domains file
./expirybot -file /path/to/domains.txt

# Add a domain to check with a threshold of 14 days (default)
./expirybot -add sheran.sg

# Add a domain to check with a custom threshold 
./expirybot -add sheran.sg,7
```

### Configuration File Format

The configuration file should contain one domain name per line. Empty lines and lines starting with `#` are ignored.

Example configuration file:

```
# List of domains to check
example.com,14
google.com,7
github.com,14
```

### Output Format

The application uses the following output format:

- For domains with certificates expiring within the threshold:
  ```
  [✓] domain.com - Certificate expires in X days
  ```

- For domains with errors:
  ```
  [✗] domain.com - Error message
  ```

## Default Configuration Location

Following the XDG Base Directory Specification:

- Linux/macOS: `~/.config/expirybot/expirybot.conf`
- If `XDG_CONFIG_HOME` is set: `$XDG_CONFIG_HOME/expirybot/expirybot.conf`

## Development

### Project Structure

```
expirybot/
├── main.go       # Main application code
├── domains.txt   # Sample domains file for testing
├── README.md     # Documentation
└── go.mod        # Go module file
```

### Adding More Features

The application is designed to be extensible. Some ideas for future enhancements:

- Add support for checking multiple ports
- Add JSON/CSV output formats
- Add email notifications for expiring certificates
- Add Windows support

## License

This project is open source and available under the GNU GPLv3 License.
