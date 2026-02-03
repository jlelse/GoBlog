package activitypub

// ObjectNew creates a new Object with the given type
func ObjectNew(typ ActivityType) *Object {
	return &Object{
		Type: typ,
	}
}

// PersonNew creates a new Person with the given ID
func PersonNew(id IRI) *Actor {
	return &Actor{
		Object: Object{
			Type: PersonType,
			ID:   id,
		},
	}
}

// CollectionNew creates a new Collection with the given ID
func CollectionNew(id IRI) *Collection {
	return &Collection{
		Object: Object{
			Type: CollectionType,
			ID:   id,
		},
	}
}

// ActivityNew creates a new Activity with the given type, ID and object
func ActivityNew(typ ActivityType, id IRI, obj Item) *Activity {
	return &Activity{
		Type:   typ,
		ID:     id,
		Object: obj,
	}
}
