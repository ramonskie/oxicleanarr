package integration

import (
	"fmt"
	"testing"
	
	"gopkg.in/yaml.v3"
)

func TestYAMLRoundTrip(t *testing.T) {
	input := `admin:
  password: adminpassword
rules:
  movie_retention: 7d
  tv_retention: 120d

advanced_rules:
  - name: "Test Deletion Tag"
    type: tag
    enabled: true
    tag: test-deletion
    retention: 0d
    require_watched: false
`
	var config map[string]interface{}
	if err := yaml.Unmarshal([]byte(input), &config); err != nil {
		t.Fatal("Error:", err)
	}
	
	// Modify movie_retention like UpdateRetentionPolicy does
	rules, ok := config["rules"].(map[string]interface{})
	if !ok {
		t.Fatalf("rules not map[string]interface{}, got %T", config["rules"])
	}
	rules["movie_retention"] = "999d"
	
	out, err := yaml.Marshal(config)
	if err != nil {
		t.Fatal("Marshal error:", err)
	}
	fmt.Println("Output YAML:")
	fmt.Println(string(out))
	
	// Check advanced_rules preserved
	ar, ok := config["advanced_rules"]
	if !ok {
		t.Fatal("advanced_rules lost!")
	}
	fmt.Printf("advanced_rules type: %T\n", ar)
	fmt.Printf("advanced_rules value: %+v\n", ar)
}
