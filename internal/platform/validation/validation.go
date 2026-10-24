// Package validation reproduce las validaciones declarativas de Bean Validation
// (@NotBlank, @Size, @Email, @Pattern) con los mismos mensajes por campo, en
// español. El primer error de cada campo gana, como el putIfAbsent de la versión
// en Java. El mapa resultante es el campo "fields" de la respuesta de error 400.
package validation

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// Fields asocia el nombre del campo (en JSON) con su mensaje de error.
type Fields map[string]string

// emailPattern es una comprobación laxa equivalente a la de @Email de Hibernate.
var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+$`)

// Builder acumula los errores de validación de una solicitud.
type Builder struct {
	fields Fields
}

// New crea un validador vacío.
func New() *Builder {
	return &Builder{fields: Fields{}}
}

func (b *Builder) addIfAbsent(field, message string) {
	if _, ok := b.fields[field]; !ok {
		b.fields[field] = message
	}
}

// NotBlank exige un valor no vacío (ignora espacios).
func (b *Builder) NotBlank(field, value, message string) *Builder {
	if strings.TrimSpace(value) == "" {
		b.addIfAbsent(field, message)
	}
	return b
}

// Size exige una longitud (en caracteres) entre min y max inclusive.
func (b *Builder) Size(field, value string, min, max int, message string) *Builder {
	n := utf8.RuneCountInString(value)
	if n < min || n > max {
		b.addIfAbsent(field, message)
	}
	return b
}

// MaxSize exige una longitud máxima (en caracteres).
func (b *Builder) MaxSize(field, value string, max int, message string) *Builder {
	if utf8.RuneCountInString(value) > max {
		b.addIfAbsent(field, message)
	}
	return b
}

// Email exige un correo válido; un valor vacío se considera válido (lo cubre NotBlank).
func (b *Builder) Email(field, value, message string) *Builder {
	if value != "" && !emailPattern.MatchString(value) {
		b.addIfAbsent(field, message)
	}
	return b
}

// Pattern exige que el valor cumpla la expresión regular.
func (b *Builder) Pattern(field, value string, pattern *regexp.Regexp, message string) *Builder {
	if !pattern.MatchString(value) {
		b.addIfAbsent(field, message)
	}
	return b
}

// Required exige que un valor esté presente (equivalente a @NotNull).
func (b *Builder) Required(field string, present bool, message string) *Builder {
	if !present {
		b.addIfAbsent(field, message)
	}
	return b
}

// DecimalRange exige que un número opcional esté dentro de [min, max]
// (equivalente a @DecimalMin/@DecimalMax); nil se considera válido.
func (b *Builder) DecimalRange(field string, value *float64, min, max float64, message string) *Builder {
	if value != nil && (*value < min || *value > max) {
		b.addIfAbsent(field, message)
	}
	return b
}

// Add registra un error a medida (primer error por campo gana).
func (b *Builder) Add(field, message string) *Builder {
	b.addIfAbsent(field, message)
	return b
}

// OK indica si no hubo errores.
func (b *Builder) OK() bool {
	return len(b.fields) == 0
}

// Result devuelve el mapa de errores (vacío si todo es válido).
func (b *Builder) Result() Fields {
	return b.fields
}
