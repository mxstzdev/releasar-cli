package ui

import (
	"github.com/charmbracelet/huh"
)

// Confirm shows a yes/no prompt and returns the user's choice.
func Confirm(title, description string) (bool, error) {
	var confirmed bool
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(title).
				Description(description).
				Value(&confirmed),
		),
	).WithTheme(theme()).Run(); err != nil {
		return false, err
	}
	return confirmed, nil
}

// Input shows a single-line text input and returns the entered value.
// An optional validate function is called on each keystroke to surface inline errors.
func Input(title, description, placeholder string, validate ...func(string) error) (string, error) {
	var value string
	field := huh.NewInput().
		Title(title).
		Description(description).
		Placeholder(placeholder).
		Value(&value)
	if len(validate) > 0 {
		field = field.Validate(validate[0])
	}
	if err := huh.NewForm(
		huh.NewGroup(field),
	).WithTheme(theme()).Run(); err != nil {
		return "", err
	}
	return value, nil
}

// TextArea shows a multiline text input and returns the entered value.
func TextArea(title, description string) (string, error) {
	var value string
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewText().
				Title(title).
				Description(description).
				Value(&value),
		),
	).WithTheme(theme()).Run(); err != nil {
		return "", err
	}
	return value, nil
}

func selectOne[T comparable](title, description string, options []huh.Option[T]) (T, error) {
	var selected T
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[T]().
				Title(title).
				Description(description).
				Options(options...).
				Value(&selected),
		),
	).WithTheme(theme()).Run(); err != nil {
		return selected, err
	}
	return selected, nil
}

// SelectString shows a single-select prompt and returns the chosen string value.
func SelectString(title, description string, options []huh.Option[string]) (string, error) {
	return selectOne(title, description, options)
}

// SelectInt shows a single-select prompt and returns the chosen int value.
func SelectInt(title, description string, options []huh.Option[int]) (int, error) {
	return selectOne(title, description, options)
}

func multiSelectOne[T comparable](title, description string, options []huh.Option[T]) ([]T, error) {
	var selected []T
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[T]().
				Title(title).
				Description(description).
				Options(options...).
				Value(&selected),
		),
	).WithTheme(theme()).Run(); err != nil {
		return nil, err
	}
	return selected, nil
}

// MultiSelectString shows a multi-select prompt and returns the chosen string values.
func MultiSelectString(title, description string, options []huh.Option[string]) ([]string, error) {
	return multiSelectOne(title, description, options)
}

// MultiSelectInt shows a multi-select prompt and returns the chosen int values.
func MultiSelectInt(title, description string, options []huh.Option[int]) ([]int, error) {
	return multiSelectOne(title, description, options)
}
