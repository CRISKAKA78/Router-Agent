package device

import (
	"testing"
	"time"
)

func TestCollectionSnapshotIsolation(t *testing.T) {
	s, _ := New(2)
	r := Registration{DeviceID: "id", Template: &TemplateReference{ID: "t", Name: "T", Version: 1}, Attributes: map[string]Attribute{"x": {Name: "X", Value: "first"}}, CollectionErrors: map[string]CollectionError{"y": {Name: "Y", Reason: "empty"}}}
	s.Publish(r, "one", time.Now())
	r.Template.Name = "mutated"
	r.Attributes["x"] = Attribute{}
	delete(r.CollectionErrors, "y")
	v, _ := s.Get("id")
	if v.Registration.Template.Name != "T" || v.Registration.Attributes["x"].Value != "first" || len(v.Registration.CollectionErrors) != 1 {
		t.Fatal(v)
	}
	v.Registration.Template.Name = "again"
	v.Registration.Attributes["x"] = Attribute{}
	s.Publish(Registration{DeviceID: "id"}, "two", time.Now())
	history, _ := s.Sessions("id")
	if history.Ended[0].Registration.Template.Name != "T" || history.Ended[0].Registration.Attributes["x"].Value != "first" || history.Current.Registration.Template != nil {
		t.Fatal(history)
	}
}
