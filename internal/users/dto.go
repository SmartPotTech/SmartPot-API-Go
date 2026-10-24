package users

import (
	"regexp"
	"time"

	"github.com/SmartPotTech/SmartPot-API-Go/internal/platform/validation"
)

// personName admite letras (con tildes y ñ), espacios, apóstrofo y guion.
var personName = regexp.MustCompile(`^[\p{L}][\p{L} '\-]*$`)

// UserResponse es la vista pública de una cuenta.
type UserResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	LastName  string    `json:"lastName"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

// ToResponse mapea la entidad a su vista pública.
func ToResponse(u *User) UserResponse {
	return UserResponse{
		ID:        u.HexID(),
		Name:      u.Name,
		LastName:  u.LastName,
		Email:     u.Email,
		Role:      string(u.Role),
		CreatedAt: u.CreatedAt,
	}
}

// UpdateProfileRequest actualiza el nombre y el apellido.
type UpdateProfileRequest struct {
	Name     string `json:"name"`
	LastName string `json:"lastName"`
}

// Validate aplica las mismas reglas y mensajes que la versión en Java.
func (r UpdateProfileRequest) Validate() validation.Fields {
	return validation.New().
		NotBlank("name", r.Name, "El nombre es obligatorio").
		Size("name", r.Name, 2, 40, "El nombre debe tener entre 2 y 40 caracteres").
		Pattern("name", r.Name, personName, "El nombre solo puede tener letras y espacios").
		NotBlank("lastName", r.LastName, "El apellido es obligatorio").
		Size("lastName", r.LastName, 2, 60, "El apellido debe tener entre 2 y 60 caracteres").
		Pattern("lastName", r.LastName, personName, "El apellido solo puede tener letras y espacios").
		Result()
}

// ChangePasswordRequest cambia la contraseña de la cuenta.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// Validate exige que ambos campos estén presentes (la política se aplica aparte).
func (r ChangePasswordRequest) Validate() validation.Fields {
	return validation.New().
		NotBlank("currentPassword", r.CurrentPassword, "La contraseña actual es obligatoria").
		NotBlank("newPassword", r.NewPassword, "La nueva contraseña es obligatoria").
		Result()
}
