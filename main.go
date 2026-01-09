package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mdview/archive"
	"mdview/browser"
	"mdview/converter"
	"mdview/output"
	"mdview/register"
	"mdview/templates"
)

const version = "1.1.2"

func main() {
	// Define flags
	templateName := flag.String("template", "default", "Template name to use for styling")
	showVersion := flag.Bool("version", false, "Show version information")
	listTemplates := flag.Bool("list-templates", false, "List available templates")
	noBrowser := flag.Bool("no-browser", false, "Don't open browser after conversion")
	selfContained := flag.Bool("self-contained", false, "Embed images and linked local .md files as base64 data URIs instead of file:// URLs")
	preload := flag.Bool("preload", false, "Preload all images in a directory when first image is referenced (use with --self-contained)")
	maxPages := flag.Int("max-pages", 10, "Maximum number of pages to embed in archive (use with --self-contained)")
	doRegister := flag.Bool("register", false, "Register mdview as the default program for .md files")
	doUnregister := flag.Bool("unregister", false, "Unregister mdview as the default program for .md files")

	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "mdview - Markdown to HTML viewer\n\n")
		fmt.Fprintf(os.Stderr, "Usage: mdview [options] <input.md> [output.html]\n\n")
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  input.md      Path to the markdown file to convert\n")
		fmt.Fprintf(os.Stderr, "  output.html   Optional output path (default: temp file in %%LocalAppData%%\\mdview)\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.VisitAll(func(f *flag.Flag) {
			fmt.Fprintf(os.Stderr, "  --%s", f.Name)
			if f.DefValue != "false" && f.DefValue != "" {
				fmt.Fprintf(os.Stderr, " %s", f.DefValue)
			}
			fmt.Fprintf(os.Stderr, "\n        %s\n", f.Usage)
		})
	}

	flag.Parse()

	// Handle version flag
	if *showVersion {
		fmt.Printf("mdview version %s\n", version)
		os.Exit(0)
	}

	// Handle register flag
	if *doRegister {
		if err := register.Register(); err != nil {
			fmt.Fprintf(os.Stderr, "Error registering: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("mdview registered as default program for .md files")
		os.Exit(0)
	}

	// Handle unregister flag
	if *doUnregister {
		if err := register.Unregister(); err != nil {
			fmt.Fprintf(os.Stderr, "Error unregistering: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("mdview unregistered as default program for .md files")
		os.Exit(0)
	}

	// Handle list-templates flag
	if *listTemplates {
		names, err := templates.List()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing templates: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Available templates:")
		for _, name := range names {
			fmt.Printf("  - %s\n", name)
		}
		os.Exit(0)
	}

	// Get positional arguments
	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Error: input markdown file is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	inputPath := args[0]
	var outputPath string
	if len(args) >= 2 {
		outputPath = args[1]
	}

	// Validate input file exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: input file does not exist: %s\n", inputPath)
		os.Exit(1)
	}

	// Validate template exists
	if _, err := templates.Get(*templateName); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Use --list-templates to see available templates\n")
		os.Exit(1)
	}

	// Run the conversion
	if err := run(inputPath, outputPath, *templateName, !*noBrowser, *selfContained, *preload, *maxPages); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(inputPath, outputPath, templateName string, openBrowser, selfContained, preload bool, maxPages int) error {
	// Determine output path
	finalOutputPath, err := output.GetOutputPath(outputPath)
	if err != nil {
		return fmt.Errorf("failed to determine output path: %w", err)
	}

	// Make input path absolute for better error messages
	absInputPath, err := filepath.Abs(inputPath)
	if err != nil {
		return fmt.Errorf("failed to resolve input path: %w", err)
	}

	// If self-contained, check if document has links to other .md files
	if selfContained {
		hasMarkdownLinks, err := archive.HasMarkdownLinks(absInputPath)
		if err != nil {
			// Don't fail, just log warning and continue with single-file conversion
			fmt.Fprintf(os.Stderr, "Warning: failed to check for markdown links: %v\n", err)
		} else if hasMarkdownLinks {
			// Use archive converter for multi-page archive
			return runArchiveConversion(absInputPath, finalOutputPath, templateName, openBrowser, selfContained, preload, maxPages)
		}
	}

	// Fall back to single-file conversion
	return runSingleFileConversion(absInputPath, finalOutputPath, templateName, openBrowser, selfContained, preload)
}

func runArchiveConversion(absInputPath, finalOutputPath, templateName string, openBrowser, selfContained, preload bool, maxPages int) error {
	// Use archive writer helper function
	err := archive.WriteArchive(absInputPath, finalOutputPath, templateName, maxPages, selfContained, preload)
	if err != nil {
		return err
	}

	// Print output path
	fmt.Printf("Generated: %s\n", finalOutputPath)

	// Open in browser if requested
	if openBrowser {
		if err := browser.Open(finalOutputPath); err != nil {
			// Don't fail on browser error, just warn
			fmt.Fprintf(os.Stderr, "Warning: failed to open browser: %v\n", err)
		}
	}

	return nil
}

func runSingleFileConversion(absInputPath, finalOutputPath, templateName string, openBrowser, selfContained, preload bool) error {
	// Open input file for streaming read
	inputFile, err := os.Open(absInputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inputFile.Close()

	// Get file size for optimal buffer pre-allocation
	var fileSize int64
	if stat, err := inputFile.Stat(); err == nil {
		fileSize = stat.Size()
	}

	// Create output file for streaming write
	outputFile, err := os.Create(finalOutputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	// Create converter and perform conversion with size hint
	conv := converter.New()
	conv.SetBaseDir(filepath.Dir(absInputPath))
	conv.SetSelfContained(selfContained)
	conv.SetPreload(preload)
	if err := conv.ConvertWithSize(inputFile, outputFile, templateName, fileSize); err != nil {
		// Clean up partial output file on error
		outputFile.Close()
		os.Remove(finalOutputPath)
		return fmt.Errorf("conversion failed: %w", err)
	}

	// Ensure output is flushed
	if err := outputFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync output file: %w", err)
	}

	// Print output path
	fmt.Printf("Generated: %s\n", finalOutputPath)

	// Open in browser if requested
	if openBrowser {
		if err := browser.Open(finalOutputPath); err != nil {
			// Don't fail on browser error, just warn
			fmt.Fprintf(os.Stderr, "Warning: failed to open browser: %v\n", err)
		}
	}

	return nil
}

func init() {
	// Normalize Windows paths in arguments
	// This handles paths like "C:\path\to\file.md" properly
	for i, arg := range os.Args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		// Clean the path to handle mixed separators
		os.Args[i] = filepath.Clean(arg)
	}
}
