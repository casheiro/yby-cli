package cmd

import "testing"

func TestValidateSecretKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Valid Key", "good-key", false},
		{"Valid Key with dots", "my.key", false},
		{"Valid Key with underscore", "my_key", false},
		{"Invalid Key with equals", "key=value", true},
		{"Invalid Key with space", "key value", true},
		{"Invalid Key with special char", "key@value", true},
		{"Empty Key", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSecretKey(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSecretKey(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
