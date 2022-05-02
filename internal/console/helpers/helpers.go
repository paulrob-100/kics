package helpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/Checkmarx/kics/internal/metrics"
	"github.com/Checkmarx/kics/pkg/progress"
	"github.com/Checkmarx/kics/pkg/report"
	"github.com/hashicorp/hcl"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

var reportGenerators = map[string]func(path, filename string, body interface{}) error{
	"json":        report.PrintJSONReport,
	"sarif":       report.PrintSarifReport,
	"html":        report.PrintHTMLReport,
	"glsast":      report.PrintGitlabSASTReport,
	"pdf":         report.PrintPdfReport,
	"sonarqube":   report.PrintSonarQubeReport,
	"cyclonedx":   report.PrintCycloneDxReport,
	"junit":       report.PrintJUnitReport,
	"asff":        report.PrintASFFReport,
	"csv":         report.PrintCSVReport,
	"codeclimate": report.PrintCodeClimateReport,
}

// CustomConsoleWriter creates an output to print log in a files
func CustomConsoleWriter(fileLogger *zerolog.ConsoleWriter) zerolog.ConsoleWriter {
	fileLogger.FormatLevel = func(i interface{}) string {
		return strings.ToUpper(fmt.Sprintf("| %-6s|", i))
	}

	fileLogger.FormatFieldName = func(i interface{}) string {
		return fmt.Sprintf("%s:", i)
	}

	fileLogger.FormatErrFieldName = func(i interface{}) string {
		return "ERROR:"
	}

	fileLogger.FormatFieldValue = func(i interface{}) string {
		return fmt.Sprintf("%s", i)
	}

	return *fileLogger
}

// FileAnalyzer determines the type of extension in the passed config file by its content
func FileAnalyzer(path string) (string, error) {
	ostat, err := os.Open(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	rc, err := io.ReadAll(ostat)
	if err != nil {
		return "", err
	}
	var temp map[string]interface{}

	// CxSAST query under review
	if err := json.Unmarshal(rc, &temp); err == nil {
		return "json", nil
	}

	// CxSAST query under review
	if err := yaml.Unmarshal(rc, &temp); err == nil {
		return "yaml", nil
	}

	// CxSAST query under review
	if _, err := toml.Decode(string(rc), &temp); err == nil {
		return "toml", nil
	}

	// CxSAST query under review
	if c, err := hcl.Parse(string(rc)); err == nil {
		if err = hcl.DecodeObject(&temp, c); err == nil {
			return "hcl", nil
		}
	}

	return "", errors.New("invalid configuration file format")
}

// GenerateReport execute each report function to generate report
func GenerateReport(path, filename string, body interface{}, formats []string, proBarBuilder progress.PbBuilder) error {
	log.Debug().Msgf("helpers.GenerateReport()")
	metrics.Metric.Start("generate_report")

	progressBar := proBarBuilder.BuildCircle("Generating Reports: ")

	var err error = nil
	go progressBar.Start()
	defer progressBar.Close()

	for _, format := range formats {
		format = strings.ToLower(format)
		if err = reportGenerators[format](path, filename, body); err != nil {
			log.Error().Msgf("Failed to generate %s report", format)
			break
		}
	}
	metrics.Metric.Stop()
	return err
}

// GetExecutableDirectory - returns the path to the directory containing KICS executable
func GetExecutableDirectory() string {
	log.Debug().Msg("helpers.GetExecutableDirectory()")
	path, err := os.Executable()
	if err != nil {
		log.Err(err)
	}
	return filepath.Dir(path)
}

// GetDefaultQueryPath - returns the default query path
func GetDefaultQueryPath(queriesPath string) (string, error) {
	log.Debug().Msg("helpers.GetDefaultQueryPath()")
	executableDirPath := GetExecutableDirectory()
	queriesDirectory := filepath.Join(executableDirPath, queriesPath)
	if _, err := os.Stat(queriesDirectory); os.IsNotExist(err) {
		currentWorkDir, err := os.Getwd()
		if err != nil {
			return "", err
		}
		idx := strings.Index(currentWorkDir, "kics")
		if idx != -1 {
			currentWorkDir = currentWorkDir[:strings.LastIndex(currentWorkDir, "kics")] + "kics"
		}
		queriesDirectory = filepath.Join(currentWorkDir, queriesPath)
		if _, err := os.Stat(queriesDirectory); os.IsNotExist(err) {
			return "", err
		}
	}

	log.Debug().Msgf("Queries found in %s", queriesDirectory)
	return queriesDirectory, nil
}

// ListReportFormats return a slice with all supported report formats
func ListReportFormats() []string {
	supportedFormats := make([]string, 0, len(reportGenerators))
	for reportFormats := range reportGenerators {
		supportedFormats = append(supportedFormats, reportFormats)
	}
	sort.Strings(supportedFormats)
	return supportedFormats
}
