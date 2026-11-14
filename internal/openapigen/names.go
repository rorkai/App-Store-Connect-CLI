package main

import (
	"regexp"
	"strings"
	"unicode"
)

var initialisms = map[string]string{
	"api": "API", "csr": "CSR", "html": "HTML", "http": "HTTP", "iap": "IAP", "id": "ID", "ids": "IDs",
	"ios": "IOS", "ipa": "IPA", "json": "JSON", "ms": "MS", "os": "OS", "sdk": "SDK", "sha1": "SHA1",
	"sha256": "SHA256", "sku": "SKU", "ssh": "SSH", "udid": "UDID", "uri": "URI", "uris": "URIs",
	"url": "URL", "urls": "URLs", "uuid": "UUID",
}

var wordPattern = regexp.MustCompile(`[a-z0-9]+|[A-Z][a-z0-9]*`)

// goName turns a JSON name such as "apiKeyId" into a Go name such as "APIKeyID".
func goName(jsonName string) string {
	var b strings.Builder
	for _, word := range wordPattern.FindAllString(jsonName, -1) {
		if v, ok := initialisms[strings.ToLower(word)]; ok {
			b.WriteString(v)
			continue
		}
		b.WriteString(upperFirst(word))
	}
	return b.String()
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// resourceTypeConst names the ResourceType constant for a JSON:API type.
func resourceTypeConst(jsonType string) string {
	return "ResourceType" + upperFirst(jsonType)
}

// fileName names the generated file of a tag: "GameCenterGroups" -> "game_center_groups_gen.go".
func fileName(tag string) string {
	var words []string
	for _, w := range wordPattern.FindAllString(lowerFirst(tag), -1) {
		words = append(words, strings.ToLower(w))
	}
	return strings.Join(words, "_") + "_gen.go"
}

// defaultName is the method name rule for an operation that has no client
// method yet. operationId is "<collection>[_<relationship>]_<action>":
//
//	getCollection -> Get<Collection>      getInstance -> Get<Resource>
//	createInstance -> Create<Resource>    updateInstance -> Update<Resource>
//	deleteInstance -> Delete<Resource>
//	getToManyRelated, getToOneRelated, getMetrics -> Get<Resource><Relationship>
//	getToManyRelationship -> Get<Resource><Relationship>Relationships
//	getToOneRelationship -> Get<Resource><Relationship>Relationship
//	createToManyRelationship -> Add<Resource><Relationship>
//	deleteToManyRelationship -> Remove<Resource><Relationship>
//	replaceToManyRelationship, updateToOneRelationship -> Update<Resource><Relationship>Relationship
//
// <Resource> is the singular resource schema name, with the collection's
// version suffix ("V2") kept at the end.
func defaultName(spec *Spec, op *Operation) string {
	parts := strings.Split(op.ID, "_")
	collection, action := parts[0], parts[len(parts)-1]
	relationship := ""
	if len(parts) == 3 {
		relationship = upperFirst(parts[1])
	}
	resource, version := singular(spec, collection)
	switch action {
	case "getCollection":
		return "Get" + upperFirst(collection)
	case "getInstance":
		return "Get" + resource + version
	case "createInstance":
		return "Create" + resource + version
	case "updateInstance":
		return "Update" + resource + version
	case "deleteInstance":
		return "Delete" + resource + version
	case "getToManyRelationship":
		return "Get" + resource + relationship + version + "Relationships"
	case "getToOneRelationship":
		return "Get" + resource + relationship + version + "Relationship"
	case "createToManyRelationship":
		return "Add" + resource + relationship + version
	case "deleteToManyRelationship":
		return "Remove" + resource + relationship + version
	case "replaceToManyRelationship", "updateToOneRelationship":
		return "Update" + resource + relationship + version + "Relationship"
	}
	return "Get" + resource + relationship + version
}

var versionSuffix = regexp.MustCompile(`V[0-9]+$`)

// singular finds the resource schema whose JSON:API type is the collection
// name; it falls back to trimming a plural "s".
func singular(spec *Spec, collection string) (string, string) {
	version := versionSuffix.FindString(collection)
	base := strings.TrimSuffix(collection, version)
	if name := spec.resourceNamed(base); name != "" {
		return name, version
	}
	return upperFirst(strings.TrimSuffix(base, "s")), version
}
