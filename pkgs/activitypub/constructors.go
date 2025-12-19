package activitypub

// ObjectNew creates a new Object with the given type
func ObjectNew(typ ActivityType) *Object {
	return &Object{
		Type: typ,
	}
}

// PersonNew creates a new Person with the given ID
func PersonNew(id IRI) *Person {
	return &Person{
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

// CreateNew creates a Create activity
func CreateNew(id ID, obj *Object) *Activity {
	return &Activity{
		Type:   CreateType,
		ID:     IRI(id),
		Object: obj,
	}
}

// UpdateNew creates an Update activity
func UpdateNew(id ID, obj Item) *Activity {
	return &Activity{
		Type:   UpdateType,
		ID:     IRI(id),
		Object: obj,
	}
}

// DeleteNew creates a Delete activity
func DeleteNew(id ID, obj Item) *Activity {
	return &Activity{
		Type:   DeleteType,
		ID:     IRI(id),
		Object: obj,
	}
}

// AcceptNew creates an Accept activity
func AcceptNew(id ID, obj Item) *Activity {
	return &Activity{
		Type:   AcceptType,
		ID:     IRI(id),
		Object: obj,
	}
}

// MentionNew creates a Mention
func MentionNew(id IRI) *Object {
	return &Object{
		Type: MentionType,
		ID:   id,
	}
}
