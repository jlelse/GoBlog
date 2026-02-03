package activitypub

import "fmt"

// ToObject converts an Item to an Object
func ToObject(item Item) (*Object, error) {
	if item == nil {
		return nil, fmt.Errorf("item is nil")
	}
	if !item.IsObject() {
		return nil, fmt.Errorf("item is not an object")
	}
	if obj, ok := item.(*Object); ok {
		return obj, nil
	}
	if actor, ok := item.(*Actor); ok {
		return &actor.Object, nil
	}
	if collection, ok := item.(*Collection); ok {
		return &collection.Object, nil
	}
	// For Activity, we can't easily convert to Object since it doesn't embed it
	// Return an error for now
	return nil, fmt.Errorf("cannot convert item to object")
}

// ToActivity converts an Item to an Activity
func ToActivity(item Item) (*Activity, error) {
	if item == nil {
		return nil, fmt.Errorf("item is nil")
	}
	if activity, ok := item.(*Activity); ok {
		return activity, nil
	}
	return nil, fmt.Errorf("item is not an activity")
}

// ToActor converts an Item to an Actor
func ToActor(item Item) (*Actor, error) {
	if item == nil {
		return nil, fmt.Errorf("item is nil")
	}
	if actor, ok := item.(*Actor); ok {
		return actor, nil
	}
	return nil, fmt.Errorf("item is not an actor")
}
