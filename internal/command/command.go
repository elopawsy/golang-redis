package command

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

var (
	ErrEmpty       = errors.New("commande vide")
	ErrUnknown     = errors.New("commande inconnue")
	ErrMissingArgs = errors.New("nombre d'arguments incorrect")
)

func Parse(input string) (Command, error) {
	args, err := split(input)
	if err != nil {
		return Command{}, err
	}
	if len(args) == 0 {
		return Command{}, ErrEmpty
	}

	cmd := Command{Kind: strings.ToUpper(args[0].text)}
	if cmd.Kind == KindGet && len(args) > 2 && strings.EqualFold(args[1].text, "WHERE") {
		if len(args) != 5 {
			return Command{}, fmt.Errorf("%w : GET WHERE champ opérateur valeur", ErrMissingArgs)
		}
		cmd.Kind = KindGetWhere
		cmd.Field = strings.ToLower(args[2].text)
		cmd.Operator = strings.ToLower(args[3].text)
		cmd.Operand = args[4].text
		cmd.OperandIsNumber = cmd.Field == "value" && numeric(args[4])
		if cmd.Field != "key" && cmd.Field != "value" {
			return Command{}, errors.New("champ attendu : key ou value")
		}
		switch cmd.Operator {
		case "equals", "contains", ">", ">=", "<", "<=":
		default:
			return Command{}, errors.New("opérateur inconnu")
		}
		return cmd, nil
	}
	switch cmd.Kind {
	case KindSet:
		if len(args) != 3 && len(args) != 5 {
			return Command{}, fmt.Errorf("%w : SET clé valeur [EX secondes]", ErrMissingArgs)
		}
		cmd.Key, cmd.Value = args[1].text, args[2].text
		cmd.IsNumber = numeric(args[2])
		if len(args) == 5 {
			if strings.ToUpper(args[3].text) != "EX" || strings.Trim(args[4].text, "0123456789") != "" {
				return Command{}, errors.New("expiration attendue : EX secondes (entier positif)")
			}
			cmd.TTL, err = time.ParseDuration(args[4].text + "s")
			if err != nil || cmd.TTL <= 0 {
				return Command{}, errors.New("la durée EX doit être un entier positif représentable")
			}
		}
	case KindGet, KindDelete, "DEL":
		if len(args) != 2 {
			return Command{}, fmt.Errorf("%w : %s clé", ErrMissingArgs, cmd.Kind)
		}
		cmd.Key = args[1].text
		if cmd.Kind == "DEL" {
			cmd.Kind = KindDelete
		}
	case KindPing:
		if len(args) != 1 {
			return Command{}, fmt.Errorf("%w : PING", ErrMissingArgs)
		}
	default:
		return Command{}, fmt.Errorf("%w : %s", ErrUnknown, args[0].text)
	}
	return cmd, nil
}

func numeric(value token) bool {
	if value.quoted {
		return false
	}
	n, err := strconv.ParseFloat(value.text, 64)
	return err == nil && !math.IsNaN(n) && !math.IsInf(n, 0)
}

func split(input string) ([]token, error) {
	var args []token
	var word strings.Builder
	var quote rune
	started := false
	quoted := false

	for _, char := range input {
		if quote != 0 {
			if char == quote {
				quote = 0
			} else {
				word.WriteRune(char)
			}
			continue
		}
		switch char {
		case '\'', '"':
			quote = char
			quoted = true
			started = true
		case ' ', '\t', '\r', '\n':
			if started {
				args = append(args, token{text: word.String(), quoted: quoted})
				word.Reset()
				started = false
				quoted = false
			}
		default:
			word.WriteRune(char)
			started = true
		}
	}
	if quote != 0 {
		return nil, errors.New("guillemet non fermé")
	}
	if started {
		args = append(args, token{text: word.String(), quoted: quoted})
	}
	return args, nil
}
