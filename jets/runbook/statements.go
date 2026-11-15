package runbook

import (
	"strings"

	"github.com/artisoft-io/jetstore/jets/sqlscript"
)

// Statements no runbook step may run.
//
// A step runs in one transaction the executor opens and closes, as the
// database identity the executor chose, so a step may not run: transaction
// control (BEGIN, START, COMMIT, END, ROLLBACK, ABORT, SAVEPOINT, RELEASE,
// PREPARE TRANSACTION), a change of role or session authorization or of the
// step's transaction, COPY (it reads and writes server files and can run a
// program), LOAD, DISCARD, LISTEN and UNLISTEN. A COMMIT inside a ddl would
// end the step's transaction early, so the statements after it would not be
// atomic and a preview would not roll back.
//
// The check is made where the runbook is read, not where it runs: Validate
// refuses such a statement when it has the SQL files (Load, when a run starts
// and when the runbooks are listed), so a run never starts with one, and Bind
// refuses it too, because the executor reads the files again when a paused run
// goes on and binds what it reads. Nothing that is refused reaches the
// database.

// ForbiddenStatement says why a runbook step may not run the statement sql, or
// returns "" when it may. It reads the statement's first keywords, skipping
// comments, white space and opening parentheses.
func ForbiddenStatement(sql string) string {
	w := firstWords(sql)
	switch w[0] {
	case "BEGIN", "START", "COMMIT", "END", "ROLLBACK", "ABORT", "SAVEPOINT", "RELEASE":
		return w[0] + " is transaction control: a step runs in one transaction the executor opens and closes"
	case "PREPARE":
		if w[1] == "TRANSACTION" {
			return "PREPARE TRANSACTION is transaction control"
		}
	case "SET", "RESET":
		for _, x := range w[1:] {
			switch x {
			case "ROLE", "AUTHORIZATION":
				return w[0] + " " + x + " changes the database identity a step runs as"
			case "TRANSACTION", "CHARACTERISTICS":
				return w[0] + " " + x + " changes the step's transaction"
			}
		}
		if w[0] == "RESET" && w[1] == "ALL" {
			return "RESET ALL resets the step's transaction settings"
		}
	case "COPY":
		return "COPY reads and writes server files and can run a program; a runbook does neither"
	case "LOAD":
		return "LOAD loads a shared library into the server"
	case "DISCARD", "LISTEN", "UNLISTEN":
		return w[0] + " changes the session, which a runbook does not do"
	}
	return ""
}

// firstWords is the first three keywords of a statement, upper case, skipping
// comments, white space and opening parentheses; "" for each one missing.
func firstWords(sql string) [3]string {
	var words [3]string
	n := 0
	sc := sqlscript.NewScanner(sql)
scan:
	for n < len(words) {
		tok, ok, err := sc.Next()
		if err != nil || !ok {
			break scan
		}
		switch tok.Kind {
		case sqlscript.LineComment, sqlscript.BlockComment:
			continue
		case sqlscript.Ident:
			words[n] = strings.ToUpper(tok.Text)
			n++
			continue
		case sqlscript.Other:
			if strings.TrimSpace(tok.Text) == "" || tok.Text == "(" {
				continue
			}
		}
		break scan
	}
	return words
}
