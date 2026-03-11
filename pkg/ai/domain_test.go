package ai

import "testing"

func TestGovernanceBlueprint_Fields(t *testing.T) {
	bp := GovernanceBlueprint{
		Domain:    "Fintech",
		RiskLevel: "Critical",
		Summary:   "Sistema de pagamentos",
		Files: []GeneratedFile{
			{Path: ".synapstor/00_PROJECT_OVERVIEW.md", Content: "# Overview"},
			{Path: ".synapstor/.uki/UKI_PCI_DSS.md", Content: "# PCI DSS"},
		},
	}

	if bp.Domain != "Fintech" {
		t.Errorf("expected Fintech, got %s", bp.Domain)
	}
	if bp.RiskLevel != "Critical" {
		t.Errorf("expected Critical, got %s", bp.RiskLevel)
	}
	if len(bp.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(bp.Files))
	}
	if bp.Files[0].Path != ".synapstor/00_PROJECT_OVERVIEW.md" {
		t.Errorf("unexpected path: %s", bp.Files[0].Path)
	}
	if bp.Files[1].Content != "# PCI DSS" {
		t.Errorf("unexpected content: %s", bp.Files[1].Content)
	}
}

func TestGeneratedFile_Fields(t *testing.T) {
	f := GeneratedFile{
		Path:    ".synapstor/.personas/ARCHITECT_BOT.md",
		Content: "# Architect Bot",
	}
	if f.Path == "" {
		t.Error("Path should not be empty")
	}
	if f.Content == "" {
		t.Error("Content should not be empty")
	}
}

func TestGovernanceBlueprint_EmptyFiles(t *testing.T) {
	bp := GovernanceBlueprint{
		Domain:    "E-commerce",
		RiskLevel: "High",
		Summary:   "Loja virtual",
		Files:     nil,
	}
	if len(bp.Files) != 0 {
		t.Errorf("expected 0 files, got %d", len(bp.Files))
	}
}
