package api

import "testing"

func TestStorageVisibilityPublicContract(t *testing.T) {
	_, _, base, _ := fixture(t, Config{})
	for _, value := range []string{"true", "false"} {
		created := data(request(t, base, "POST", "/api/v1/probe-templates", "storage-"+value, `{"name":"Storage-`+value+`","properties":{},"presentation":{"storage_visible":`+value+`}}`, 201))
		id := created["template_id"].(string)
		read := data(request(t, base, "GET", "/api/v1/probe-templates/"+id, "", "", 200))
		if read["presentation"].(map[string]any)["storage_visible"] != (value == "true") {
			t.Fatal("storage presentation lost", read)
		}
	}
	request(t, base, "POST", "/api/v1/probe-templates", "storage-null", `{"name":"Invalid","properties":{},"presentation":{"storage_visible":null}}`, 400)
}
