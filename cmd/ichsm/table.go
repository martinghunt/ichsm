package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func writeJSONValue(out io.Writer, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(encoded))
	return err
}

func writeRowsForOutputFormat(out io.Writer, rows [][]string, outfmt string) error {
	switch outfmt {
	case outputFormatTable:
		return writeAlignedRows(out, rows)
	case outputFormatTTable:
		return writeTransposedTable(out, rows)
	case outputFormatTTSV:
		return writeTransposedDelimitedRows(out, rows, "\t")
	case outputFormatTSV:
		return writeDelimitedRows(out, rows, "\t")
	default:
		return fmt.Errorf("unsupported tabular output format %q", outfmt)
	}
}

func dotIfEmpty(value string) string {
	if value == "" {
		return "."
	}
	return value
}

func writeAlignedRows(out io.Writer, rows [][]string) error {
	if len(rows) == 0 {
		return nil
	}

	rows = sanitizeTabularRows(rows)
	widths := make([]int, maxRowWidth(rows))
	for _, row := range rows {
		for i, value := range row {
			if len(value) > widths[i] {
				widths[i] = len(value)
			}
		}
	}

	for _, row := range rows {
		for i, value := range row {
			if i > 0 {
				fmt.Fprint(out, "  ")
			}
			fmt.Fprint(out, value)
			if i < len(row)-1 {
				fmt.Fprint(out, strings.Repeat(" ", widths[i]-len(value)))
			}
		}
		fmt.Fprintln(out)
	}
	return nil
}

func writeDelimitedRows(out io.Writer, rows [][]string, delimiter string) error {
	for _, row := range rows {
		row = sanitizeTabularRow(row)
		fmt.Fprintln(out, strings.Join(row, delimiter))
	}
	return nil
}

func writeTransposedTable(out io.Writer, rows [][]string) error {
	return writeAlignedRows(out, transposeRows(rows))
}

func writeTransposedDelimitedRows(out io.Writer, rows [][]string, delimiter string) error {
	return writeDelimitedRows(out, transposeRows(rows), delimiter)
}

func transposeRows(rows [][]string) [][]string {
	width := maxRowWidth(rows)
	if width == 0 {
		return nil
	}

	transposed := make([][]string, width)
	for columnIndex := 0; columnIndex < width; columnIndex++ {
		transposed[columnIndex] = make([]string, len(rows))
		for rowIndex, row := range rows {
			if columnIndex < len(row) {
				transposed[columnIndex][rowIndex] = row[columnIndex]
			}
		}
	}
	return transposed
}

func tsvTextRows(text string) [][]string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}

	rows := make([][]string, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, strings.Split(line, "\t"))
	}
	return rows
}

func maxRowWidth(rows [][]string) int {
	maxWidth := 0
	for _, row := range rows {
		if len(row) > maxWidth {
			maxWidth = len(row)
		}
	}
	return maxWidth
}

func sanitizeTabularRows(rows [][]string) [][]string {
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, sanitizeTabularRow(row))
	}
	return out
}

func sanitizeTabularRow(row []string) []string {
	out := make([]string, len(row))
	for i, value := range row {
		out[i] = sanitizeTabularCell(value)
	}
	return out
}

func sanitizeTabularCell(value string) string {
	var builder strings.Builder
	changed := false
	lastWasSpace := false
	for _, char := range value {
		switch char {
		case '\n', '\r', '\t':
			changed = true
			if builder.Len() > 0 && !lastWasSpace {
				builder.WriteByte(' ')
				lastWasSpace = true
			}
		default:
			builder.WriteRune(char)
			lastWasSpace = char == ' '
		}
	}
	if !changed {
		return value
	}
	return strings.TrimSpace(builder.String())
}
