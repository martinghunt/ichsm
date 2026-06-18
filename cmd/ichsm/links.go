package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/martinghunt/ichsm"
	"github.com/spf13/cobra"
)

var linkRunFields = []string{
	"run_accession",
	"experiment_accession",
	"sample_accession",
	"secondary_sample_accession",
	"study_accession",
	"secondary_study_accession",
}

var linkSampleFields = []string{
	"sample_accession",
	"secondary_sample_accession",
	"study_accession",
}

var linkStudyFields = []string{
	"study_accession",
	"secondary_study_accession",
}

var linkContigSetFields = []string{
	"accession",
	"sample_accession",
	"secondary_sample_accession",
	"study_accession",
}

var linkWGSSetFields = append(append([]string(nil), linkContigSetFields...), "run_accession")

type linkEntry struct {
	label     string
	accession string
}

type linkTreeNode struct {
	label     string
	accession string
	children  []*linkTreeNode
	childByID map[string]*linkTreeNode
}

func newLinksCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "links [accession]",
		Short: "Show linked project, sample, experiment, run, and contig set accessions",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeLinks(cmd, args[0])
		},
	}

	return cmd
}

func executeLinks(cmd *cobra.Command, accession string) error {
	accession = strings.TrimSpace(accession)
	if accession == "" {
		return fmt.Errorf("accession is required")
	}

	fixedAccession, accessionType, ok := ichsm.IdentifyAccession(accession)
	if !ok {
		return fmt.Errorf("accession format not recognised: %s", accession)
	}

	client := newClient()
	roots, err := linkTree(cmd.Context(), client, fixedAccession, accessionType)
	if err != nil {
		return err
	}
	if len(roots) == 0 {
		return fmt.Errorf("no links found for accession %s", accession)
	}

	return writeLinkTree(cmd.OutOrStdout(), roots)
}

func linkTree(ctx context.Context, client *ichsm.Client, accession string, accessionType ichsm.AccessionType) ([]*linkTreeNode, error) {
	builder := newLinkTreeBuilder()

	switch accessionType {
	case ichsm.AccessionTypeRun, ichsm.AccessionTypeExperiment:
		records, err := queryLinkRunRecords(ctx, client, accession, accessionType)
		if err != nil {
			return nil, err
		}
		sampleAccessions := recordSampleAccessions(records)
		if len(sampleAccessions) > 0 {
			for _, sampleAccession := range sampleAccessions {
				if err := addSampleNeighborhood(ctx, client, builder, sampleAccession); err != nil {
					return nil, err
				}
			}
		} else {
			for _, record := range records {
				builder.addRunRecordPath(record, accessionType, accession)
			}
			contigSetRecords, err := queryRunLinkedContigSetRecords(ctx, client, records)
			if err != nil {
				return nil, err
			}
			for _, record := range contigSetRecords {
				builder.addContigSetRecordPath(record)
			}
		}
	case ichsm.AccessionTypeSample:
		if err := addSampleNeighborhood(ctx, client, builder, accession); err != nil {
			return nil, err
		}
	case ichsm.AccessionTypeStudy:
		studyRecords, err := queryLinkStudyRecords(ctx, client, accession)
		if err != nil {
			return nil, err
		}
		for _, record := range studyRecords {
			builder.addStudyRecordPath(record, accession)
		}
		searchAccession := linkPrimaryStudyAccession(studyRecords, accession)

		runRecords, err := queryLinkRunRecords(ctx, client, searchAccession, accessionType)
		if err != nil {
			return nil, err
		}
		for _, record := range runRecords {
			builder.addRunRecordPath(record, accessionType, searchAccession)
		}

		contigSetRecords, err := queryLinkContigSetRecords(ctx, client, searchAccession, accessionType)
		if err != nil {
			return nil, err
		}
		for _, record := range contigSetRecords {
			builder.addContigSetRecordPath(record)
		}
	case ichsm.AccessionTypeContigSet, ichsm.AccessionTypeWGSSet, ichsm.AccessionTypeTSASet, ichsm.AccessionTypeTLSSet:
		contigSetRecords, err := queryLinkContigSetRecords(ctx, client, accession, accessionType)
		if err != nil {
			return nil, err
		}
		sampleAccessions := recordSampleAccessions(contigSetRecords)
		if len(sampleAccessions) > 0 {
			for _, sampleAccession := range sampleAccessions {
				if err := addSampleNeighborhood(ctx, client, builder, sampleAccession); err != nil {
					return nil, err
				}
			}
		} else if err := addContigSetRunPaths(ctx, client, builder, contigSetRecords); err != nil {
			return nil, err
		}
		for _, record := range contigSetRecords {
			builder.addContigSetRecordPath(record)
		}
	default:
		return nil, fmt.Errorf("links supports run, experiment, sample, study/project, and contig set accessions")
	}

	return builder.roots, nil
}

func queryLinkRunRecords(ctx context.Context, client *ichsm.Client, accession string, accessionType ichsm.AccessionType) ([]ichsm.Record, error) {
	_, _, records, err := client.Query(ctx, accession, accessionType, linkRunFields, ichsm.AccessionTypeRun)
	if err != nil {
		return nil, fmt.Errorf("error getting links for accession %s: %w", accession, err)
	}
	return records, nil
}

func queryLinkSampleRecords(ctx context.Context, client *ichsm.Client, accession string) ([]ichsm.Record, error) {
	_, _, records, err := client.Query(ctx, accession, ichsm.AccessionTypeSample, linkSampleFields, ichsm.AccessionTypeSample)
	if err != nil {
		return nil, fmt.Errorf("error getting links for accession %s: %w", accession, err)
	}
	return records, nil
}

func queryLinkStudyRecords(ctx context.Context, client *ichsm.Client, accession string) ([]ichsm.Record, error) {
	_, _, records, err := client.Query(ctx, accession, ichsm.AccessionTypeStudy, linkStudyFields, ichsm.AccessionTypeStudy)
	if err != nil {
		return nil, fmt.Errorf("error getting links for accession %s: %w", accession, err)
	}
	return records, nil
}

func queryLinkContigSetRecords(ctx context.Context, client *ichsm.Client, accession string, accessionType ichsm.AccessionType) ([]ichsm.Record, error) {
	levels := linkContigSetLevels(accessionType)
	records := make([]ichsm.Record, 0)
	for _, level := range levels {
		_, _, levelRecords, err := client.Query(ctx, accession, accessionType, linkContigSetFieldsForLevel(level), level)
		if err != nil {
			return nil, fmt.Errorf("error getting contig set links for accession %s: %w", accession, err)
		}
		records = append(records, levelRecords...)
	}
	return records, nil
}

func addSampleNeighborhood(ctx context.Context, client *ichsm.Client, builder *linkTreeBuilder, accession string) error {
	sampleRecords, err := queryLinkSampleRecords(ctx, client, accession)
	if err != nil {
		return err
	}
	for _, record := range sampleRecords {
		builder.addSampleRecordPath(record, accession)
	}

	runRecords, err := queryLinkRunRecords(ctx, client, accession, ichsm.AccessionTypeSample)
	if err != nil {
		return err
	}
	for _, record := range runRecords {
		builder.addRunRecordPath(record, ichsm.AccessionTypeSample, accession)
	}

	contigSetRecords, err := queryLinkContigSetRecords(ctx, client, accession, ichsm.AccessionTypeSample)
	if err != nil {
		return err
	}
	for _, record := range contigSetRecords {
		builder.addContigSetRecordPath(record)
	}
	return nil
}

func queryRunLinkedContigSetRecords(ctx context.Context, client *ichsm.Client, runRecords []ichsm.Record) ([]ichsm.Record, error) {
	records := make([]ichsm.Record, 0)
	for _, runAccession := range recordRunAccessions(runRecords) {
		runContigSetRecords, err := queryLinkContigSetRecords(ctx, client, runAccession, ichsm.AccessionTypeRun)
		if err != nil {
			return nil, err
		}
		records = append(records, runContigSetRecords...)
	}
	return records, nil
}

func addContigSetRunPaths(ctx context.Context, client *ichsm.Client, builder *linkTreeBuilder, contigSetRecords []ichsm.Record) error {
	for _, runAccession := range recordRunAccessions(contigSetRecords) {
		records, err := queryLinkRunRecords(ctx, client, runAccession, ichsm.AccessionTypeRun)
		if err != nil {
			return err
		}
		for _, record := range records {
			builder.addRunRecordPath(record, ichsm.AccessionTypeRun, runAccession)
		}
	}
	return nil
}

func recordRunAccessions(records []ichsm.Record) []string {
	seen := map[string]bool{}
	var accessions []string
	for _, record := range records {
		for _, accession := range recordLinkValues(record, "run_accession") {
			if seen[accession] {
				continue
			}
			accessions = append(accessions, accession)
			seen[accession] = true
		}
	}
	return accessions
}

func recordSampleAccessions(records []ichsm.Record) []string {
	seen := map[string]bool{}
	var accessions []string
	for _, record := range records {
		for _, accession := range recordLinkValues(record, "sample_accession") {
			if seen[accession] {
				continue
			}
			accessions = append(accessions, accession)
			seen[accession] = true
		}
	}
	return accessions
}

func linkPrimaryStudyAccession(records []ichsm.Record, fallback string) string {
	for _, record := range records {
		if accession := recordLinkString(record, "study_accession"); accession != "" {
			return accession
		}
	}
	return fallback
}

func linkContigSetLevels(accessionType ichsm.AccessionType) []ichsm.AccessionType {
	switch accessionType {
	case ichsm.AccessionTypeRun:
		return []ichsm.AccessionType{ichsm.AccessionTypeWGSSet}
	case ichsm.AccessionTypeSample, ichsm.AccessionTypeStudy, ichsm.AccessionTypeContigSet:
		return []ichsm.AccessionType{ichsm.AccessionTypeWGSSet, ichsm.AccessionTypeTSASet, ichsm.AccessionTypeTLSSet}
	case ichsm.AccessionTypeWGSSet:
		return []ichsm.AccessionType{ichsm.AccessionTypeWGSSet}
	case ichsm.AccessionTypeTSASet:
		return []ichsm.AccessionType{ichsm.AccessionTypeTSASet}
	case ichsm.AccessionTypeTLSSet:
		return []ichsm.AccessionType{ichsm.AccessionTypeTLSSet}
	default:
		return nil
	}
}

func linkContigSetFieldsForLevel(level ichsm.AccessionType) []string {
	if level == ichsm.AccessionTypeWGSSet {
		return linkWGSSetFields
	}
	return linkContigSetFields
}

type linkTreeBuilder struct {
	roots    []*linkTreeNode
	rootByID map[string]*linkTreeNode
}

func newLinkTreeBuilder() *linkTreeBuilder {
	return &linkTreeBuilder{rootByID: map[string]*linkTreeNode{}}
}

func (b *linkTreeBuilder) addStudyRecordPath(record ichsm.Record, fixedAccession string) {
	projects := recordLinkValues(record, "study_accession")
	if len(projects) == 0 {
		projects = recordLinkValues(record, "secondary_study_accession")
	}
	if len(projects) == 0 {
		projects = []string{fixedAccession}
	}
	for _, project := range projects {
		b.addPath(project, "", "", "", "")
	}
}

func (b *linkTreeBuilder) addSampleRecordPath(record ichsm.Record, fixedAccession string) {
	sample := firstNonEmpty(recordLinkString(record, "sample_accession", "secondary_sample_accession"), fixedAccession)
	projects := recordLinkValues(record, "study_accession")
	if len(projects) == 0 {
		projects = []string{""}
	}
	for _, project := range projects {
		b.addPath(project, sample, "", "", "")
	}
}

func (b *linkTreeBuilder) addRunRecordPath(record ichsm.Record, accessionType ichsm.AccessionType, fixedAccession string) {
	projects := recordLinkValues(record, "study_accession")
	if len(projects) == 0 {
		projects = recordLinkValues(record, "secondary_study_accession")
	}
	if len(projects) == 0 {
		projects = []string{""}
	}

	sample := recordLinkString(record, "sample_accession", "secondary_sample_accession")
	experiment := recordLinkString(record, "experiment_accession")
	run := recordLinkString(record, "run_accession")
	switch accessionType {
	case ichsm.AccessionTypeSample:
		sample = firstNonEmpty(sample, fixedAccession)
	case ichsm.AccessionTypeExperiment:
		experiment = firstNonEmpty(experiment, fixedAccession)
	case ichsm.AccessionTypeRun:
		run = firstNonEmpty(run, fixedAccession)
	}

	for _, project := range projects {
		b.addPath(project, sample, experiment, run, "")
	}
}

func (b *linkTreeBuilder) addContigSetRecordPath(record ichsm.Record) {
	projects := recordLinkValues(record, "study_accession")
	if len(projects) == 0 {
		projects = []string{""}
	}
	sample := recordLinkString(record, "sample_accession", "secondary_sample_accession")
	contigSet := recordLinkString(record, "accession")
	for _, project := range projects {
		b.addPath(project, sample, "", "", contigSet)
	}
}

func (b *linkTreeBuilder) addPath(project string, sample string, experiment string, run string, contigSet string) {
	entries := compactLinkEntries([]linkEntry{
		{label: "Project", accession: project},
		{label: "Sample", accession: sample},
		{label: "Experiment", accession: experiment},
		{label: "Run", accession: run},
		{label: "ContigSet", accession: contigSet},
	})
	if len(entries) == 0 {
		return
	}

	node := b.root(entries[0].label, entries[0].accession)
	for _, entry := range entries[1:] {
		node = node.child(entry.label, entry.accession)
	}
}

func (b *linkTreeBuilder) root(label string, accession string) *linkTreeNode {
	id := linkNodeID(label, accession)
	if node, ok := b.rootByID[id]; ok {
		return node
	}
	node := newLinkTreeNode(label, accession)
	b.roots = append(b.roots, node)
	b.rootByID[id] = node
	return node
}

func newLinkTreeNode(label string, accession string) *linkTreeNode {
	return &linkTreeNode{
		label:     label,
		accession: accession,
		childByID: map[string]*linkTreeNode{},
	}
}

func (n *linkTreeNode) child(label string, accession string) *linkTreeNode {
	id := linkNodeID(label, accession)
	if child, ok := n.childByID[id]; ok {
		return child
	}
	child := newLinkTreeNode(label, accession)
	n.children = append(n.children, child)
	n.childByID[id] = child
	return child
}

func linkNodeID(label string, accession string) string {
	return label + "\x00" + accession
}

func compactLinkEntries(entries []linkEntry) []linkEntry {
	out := make([]linkEntry, 0, len(entries))
	for _, entry := range entries {
		entry.accession = strings.TrimSpace(entry.accession)
		if entry.accession == "" {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func recordLinkValues(record ichsm.Record, key string) []string {
	value, ok := record[key]
	if !ok || value == nil {
		return nil
	}
	return splitLinkValues(fmt.Sprint(value))
}

func splitLinkValues(value string) []string {
	seen := map[string]bool{}
	var values []string
	for _, part := range strings.Split(value, ";") {
		part = strings.TrimSpace(part)
		if part == "" || part == "." || seen[part] {
			continue
		}
		values = append(values, part)
		seen[part] = true
	}
	return values
}

func recordLinkString(record ichsm.Record, keys ...string) string {
	for _, key := range keys {
		values := recordLinkValues(record, key)
		if len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func writeLinkTree(out io.Writer, roots []*linkTreeNode) error {
	for _, root := range roots {
		if _, err := fmt.Fprintf(out, "%s: %s\n", root.label, root.accession); err != nil {
			return err
		}
		if err := writeLinkTreeChildren(out, root.children, ""); err != nil {
			return err
		}
	}
	return nil
}

func writeLinkTreeChildren(out io.Writer, nodes []*linkTreeNode, prefix string) error {
	for i, node := range nodes {
		last := i == len(nodes)-1
		connector := "\u251c\u2500\u2500 "
		childPrefix := prefix + "\u2502   "
		if last {
			connector = "\u2514\u2500\u2500 "
			childPrefix = prefix + "    "
		}
		if _, err := fmt.Fprintf(out, "%s%s%s: %s\n", prefix, connector, node.label, node.accession); err != nil {
			return err
		}
		if err := writeLinkTreeChildren(out, node.children, childPrefix); err != nil {
			return err
		}
	}
	return nil
}
