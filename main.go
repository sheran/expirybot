package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// Default threshold in days
	defaultThreshold = 14

	// Default application name
	appName = "expirybot"

	// Timeout for domain checks
	checkTimeout = 10 * time.Second

	// Maximum number of concurrent domain checks
	maxConcurrentChecks = 10
)

func main() {
	// Define command line flags
	filePtr := flag.String("file", "", "Path to domains file (overrides default config file)")
	addDomainPtr := flag.String("add", "", "Add a domain to check (format: domain.com[,threshold])")

	// Parse command line flags
	flag.Parse()

	// Handle add domain flag
	if *addDomainPtr != "" {
		addDomain(*addDomainPtr)
		return
	}

	// Get the domains file path
	var domainsFilePath string
	if *filePtr != "" {
		// Use provided file path
		domainsFilePath = *filePtr
	} else {
		// Use XDG config directory
		domainsFilePath = getXDGConfigFilePath()
	}

	// Check if the file exists
	if _, err := os.Stat(domainsFilePath); os.IsNotExist(err) {
		fmt.Println("No domains configured to check, add a new domain with: expirybot -add domain.com[,threshold]")
		os.Exit(0)
	}

	// Read domains from file
	domains, err := readDomainsFromFile(domainsFilePath)
	if err != nil {
		fmt.Printf("Error reading domains file: %v\n", err)
		os.Exit(1)
	}

	if len(domains) == 0 {
		fmt.Println("No domains configured to check, add a new domain with: expirybot -add domain.com[,threshold]")
		os.Exit(0)
	}

	// Check domains in parallel
	checkDomainsParallel(domains, defaultThreshold)
}

// Domain represents a domain with its custom threshold
type Domain struct {
	Name      string
	Threshold int
}

// addDomain adds a domain to the configuration file
func addDomain(domainInput string) {
	parts := strings.Split(domainInput, ",")

	domain := parts[0]
	threshold := defaultThreshold

	// Check if threshold is provided
	if len(parts) > 1 {
		var err error
		threshold, err = strconv.Atoi(parts[1])
		if err != nil {
			fmt.Printf("Invalid threshold value: %s. Using default: %d days\n", parts[1], defaultThreshold)
			threshold = defaultThreshold
		}
	}

	// Get config file path
	configPath := getXDGConfigFilePath()

	// Create config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0755); err != nil {
			fmt.Printf("Error creating config directory: %v\n", err)
			os.Exit(1)
		}
	}

	// Check if domain already exists in config
	var domains []Domain
	if _, err := os.Stat(configPath); err == nil {
		domains, _ = readDomainsFromFile(configPath)

		// Check if domain already exists
		for i, d := range domains {
			if d.Name == domain {
				// Update threshold
				domains[i].Threshold = threshold
				fmt.Printf("Updated domain %s with threshold %d days\n", domain, threshold)

				// Write updated domains to file
				writeDomainsToFile(configPath, domains)
				return
			}
		}
	}

	// Add new domain
	domains = append(domains, Domain{Name: domain, Threshold: threshold})

	// Write domains to file
	writeDomainsToFile(configPath, domains)

	fmt.Printf("Added domain %s with threshold %d days\n", domain, threshold)
}

// writeDomainsToFile writes domains to the configuration file
func writeDomainsToFile(filePath string, domains []Domain) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, domain := range domains {
		_, err := writer.WriteString(fmt.Sprintf("%s,%d\n", domain.Name, domain.Threshold))
		if err != nil {
			return err
		}
	}

	return writer.Flush()
}

// getXDGConfigFilePath returns the path to the config file following XDG Base Directory Specification
func getXDGConfigFilePath() string {
	// Check XDG_CONFIG_HOME environment variable first
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		// If not set, use default ~/.config
		homeDir, err := os.UserHomeDir()
		if err != nil {
			fmt.Printf("Error getting home directory: %v\n", err)
			os.Exit(1)
		}
		configHome = filepath.Join(homeDir, ".config")
	}

	// App config directory
	appConfigDir := filepath.Join(configHome, appName)

	return filepath.Join(appConfigDir, appName+".conf")
}

// readDomainsFromFile reads domains from a file with format "domain,threshold"
func readDomainsFromFile(filePath string) ([]Domain, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var domains []Domain
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, ",")
		domain := parts[0]
		threshold := defaultThreshold

		// Parse threshold if provided
		if len(parts) > 1 {
			var err error
			threshold, err = strconv.Atoi(parts[1])
			if err != nil {
				// Use default threshold if parsing fails
				threshold = defaultThreshold
			}
		}

		domains = append(domains, Domain{Name: domain, Threshold: threshold})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return domains, nil
}

// checkDomainsParallel checks multiple domains in parallel with a limit on concurrent checks
func checkDomainsParallel(domains []Domain, defaultThreshold int) {
	// Create a semaphore channel to limit concurrent goroutines
	sem := make(chan struct{}, maxConcurrentChecks)
	var wg sync.WaitGroup

	for _, domain := range domains {
		// Use domain-specific threshold if available, otherwise use default
		threshold := domain.Threshold
		if threshold <= 0 {
			threshold = defaultThreshold
		}

		wg.Add(1)

		// Launch goroutine for each domain
		go func(domain Domain, threshold int) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }() // Release semaphore

			checkDomain(domain.Name, threshold)
		}(domain, threshold)
	}

	// Wait for all goroutines to complete
	wg.Wait()
}

// isDomainReachable checks if a domain is reachable via DNS lookup
func isDomainReachable(domain string) error {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()

	resolver := net.Resolver{}
	_, err := resolver.LookupHost(ctx, domain)
	return err
}

// checkCertificate checks the SSL certificate validity and returns expiration information
func checkCertificate(cert *x509.Certificate) (bool, int) {
	// Check if certificate is valid
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return false, 0
	}

	if now.After(cert.NotAfter) {
		return false, 0
	}

	// Calculate days until expiration
	expiresIn := time.Until(cert.NotAfter)
	expiresInDays := int(expiresIn.Hours() / 24)

	return true, expiresInDays
}

// checkDomain checks if a domain is reachable and its SSL certificate validity
func checkDomain(domain string, thresholdDays int) {
	// Check if domain is reachable
	if err := isDomainReachable(domain); err != nil {
		fmt.Printf("[✗] %s - DNS lookup failed\n", domain)
		return
	}

	// Create a connection with timeout
	dialer := &net.Dialer{
		Timeout: checkTimeout,
	}

	// Configure TLS connection
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
	}

	// Check SSL certificate
	conn, err := tls.DialWithDialer(dialer, "tcp", domain+":443", tlsConfig)
	if err != nil {
		fmt.Printf("[✗] %s - SSL connection failed or cert expired [%s]\n", domain, err)
		return
	}
	defer conn.Close()

	// Get certificate details
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		fmt.Printf("[✗] %s - No certificates found\n", domain)
		return
	}

	// Check primary certificate
	cert := certs[0]
	valid, daysRemaining := checkCertificate(cert)

	if !valid {
		fmt.Printf("[✗] %s - Certificate is not valid\n", domain)
		return
	}

	// Only print if certificate expires within threshold
	if daysRemaining <= thresholdDays {
		fmt.Printf("[✓] %s - Certificate expires in %d days\n", domain, daysRemaining)
	}
}
