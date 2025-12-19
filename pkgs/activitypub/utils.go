package activitypub

import (
	"encoding/json"
	"fmt"
)

// UnmarshalJSON unmarshals JSON data into an Item
func UnmarshalJSON(data []byte) (Item, error) {
	// First, peek at the type
	var peek struct {
		Type ActivityType `json:"type"`
	}
	if err := json.Unmarshal(data, &peek); err != nil {
		// Might be an IRI string
		var iri string
		if err := json.Unmarshal(data, &iri); err == nil {
			return IRI(iri), nil
		}
		return nil, err
	}

	// Based on type, unmarshal into the appropriate struct
	switch peek.Type {
	case PersonType:
		var person Person
		if err := json.Unmarshal(data, &person); err != nil {
			return nil, err
		}
		return &person, nil
	case CreateType, UpdateType, DeleteType, FollowType, AcceptType, UndoType, AnnounceType, LikeType, BlockType, ActivityObjType:
		var activity Activity
		if err := json.Unmarshal(data, &activity); err != nil {
			return nil, err
		}
		return &activity, nil
	case CollectionType:
		var collection Collection
		if err := json.Unmarshal(data, &collection); err != nil {
			return nil, err
		}
		return &collection, nil
	default:
		// Default to Object for unknown or generic types
		var obj Object
		if err := json.Unmarshal(data, &obj); err != nil {
			return nil, err
		}
		return &obj, nil
	}
}

// IsObject checks if an item is an object (not just an IRI)
func IsObject(item Item) bool {
	return item != nil && item.IsObject()
}

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
	if person, ok := item.(*Person); ok {
		return &person.Object, nil
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

// OnActor calls a function if the item is an actor
func OnActor(item Item, fn func(*Actor) error) error {
	if actor, ok := item.(*Actor); ok {
		return fn(actor)
	}
	return fmt.Errorf("item is not an actor")
}

// OnObject calls a function if the item is an object
func OnObject(item Item, fn func(*Object) error) error {
	obj, err := ToObject(item)
	if err != nil {
		return err
	}
	return fn(obj)
}
