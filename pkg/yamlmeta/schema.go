// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package yamlmeta

import (
	"fmt"
	"github.com/k14s/ytt/pkg/structmeta"
	"github.com/k14s/ytt/pkg/template"
)

type Schema interface {
	AssignType(typeable Typeable) TypeCheck
}

const (
	AnnotationSchemaNullable structmeta.AnnotationName = "schema/nullable"
)

func schemaAnnotationsList() []structmeta.AnnotationName {
	return []structmeta.AnnotationName{AnnotationSchemaNullable}
}

var _ Schema = &AnySchema{}
var _ Schema = &DocumentSchema{}

type AnySchema struct {
}

type DocumentSchema struct {
	Name    string
	Source  *Document
	Allowed *DocumentType
}

func NewDocumentSchema(doc *Document) (*DocumentSchema, error) {
	docType := &DocumentType{Source: doc}

	switch typedDocumentValue := doc.Value.(type) {
	case *Map:
		valueType, err := NewMapType(typedDocumentValue)
		if err != nil {
			return nil, err
		}

		docType.ValueType = valueType
	case *Array:
		valueType, err := NewArrayType(typedDocumentValue)
		if err != nil {
			return nil, err
		}

		docType.ValueType = valueType
	}
	return &DocumentSchema{
		Name:    "dataValues",
		Source:  doc,
		Allowed: docType,
	}, nil
}

func NewMapType(m *Map) (*MapType, error) {
	mapType := &MapType{}

	for _, mapItem := range m.Items {
		mapItemType, err := NewMapItemType(mapItem)
		if err != nil {
			return nil, err
		}
		mapType.Items = append(mapType.Items, mapItemType)
	}
	annotations, err := schemaAnnotations(m)
	if err != nil {
		return nil, err
	}
	mapType.annotations = annotations
	return mapType, nil
}

func NewMapItemType(item *MapItem) (*MapItemType, error) {
	valueType, err := newCollectionItemValueType(item.Value)
	if err != nil {
		return nil, err
	}

	defaultValue := item.Value
	if _, ok := item.Value.(*Array); ok {
		defaultValue = &Array{}
	}

	annotations, err := schemaAnnotations(item)
	if err != nil {
		return nil, err
	}
	if _, nullable := annotations[AnnotationSchemaNullable]; nullable {
		defaultValue = nil
	}

	return &MapItemType{Key: item.Key, ValueType: valueType, DefaultValue: defaultValue, Position: item.Position, annotations: annotations}, nil
}

func NewArrayType(a *Array) (*ArrayType, error) {
	// These really are distinct use cases. In the empty list, perhaps the user is unaware that arrays must be typed. In the >1 scenario, they may be expecting the given items to be the defaults.
	if len(a.Items) == 0 {
		return nil, fmt.Errorf("Expected one item in array (describing the type of its elements) at %s", a.Position.AsCompactString())
	}
	if len(a.Items) > 1 {
		return nil, fmt.Errorf("Expected one item (found %v) in array (describing the type of its elements) at %s", len(a.Items), a.Position.AsCompactString())
	}

	arrayItemType, err := NewArrayItemType(a.Items[0])
	if err != nil {
		return nil, err
	}

	annotations, err := schemaAnnotations(a)
	if err != nil {
		return nil, err
	}

	return &ArrayType{ItemsType: arrayItemType, annotations: annotations}, nil
}

func NewArrayItemType(item *ArrayItem) (*ArrayItemType, error) {
	valueType, err := newCollectionItemValueType(item.Value)
	if err != nil {
		return nil, err
	}

	annotations, err := schemaAnnotations(item)
	if err != nil {
		return nil, err
	}
	if _, found := annotations[AnnotationSchemaNullable]; found {
		return nil, fmt.Errorf("Array items cannot be annotated with #@schema/nullable (%s). If this behaviour would be valuable, please submit an issue on https://github.com/vmware-tanzu/carvel-ytt", item.GetPosition().AsCompactString())
	}

	return &ArrayItemType{ValueType: valueType}, nil
}

func newCollectionItemValueType(collectionItemValue interface{}) (Type, error) {
	switch typedContent := collectionItemValue.(type) {
	case *Map:
		mapType, err := NewMapType(typedContent)
		if err != nil {
			return nil, err
		}
		return mapType, nil
	case *Array:
		arrayType, err := NewArrayType(typedContent)
		if err != nil {
			return nil, err
		}
		return arrayType, nil
	case string:
		return &ScalarType{Type: *new(string)}, nil
	case int:
		return &ScalarType{Type: *new(int)}, nil
	case bool:
		return &ScalarType{Type: *new(bool)}, nil
	}

	return nil, fmt.Errorf("Collection item type did not match any known types")
}

func (as *AnySchema) AssignType(typeable Typeable) TypeCheck { return TypeCheck{} }

func (s *DocumentSchema) AssignType(typeable Typeable) TypeCheck {
	return s.Allowed.AssignTypeTo(typeable)
}

func (t MapItemType) IsNullable() bool {
	_, found := t.annotations[AnnotationSchemaNullable]
	return found
}

func schemaAnnotations(node Node) (annotations template.NodeAnnotations, err error) {
	annotations = template.NodeAnnotations{}
	anns := template.NewAnnotations(node)

	for key, meta := range anns {
		for _, schAnnName := range schemaAnnotationsList() {
			if key == schAnnName {
				annotations[key] = meta
				break
			}
		}
	}
	return
}
