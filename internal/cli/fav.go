package cli

import (
	"cmp"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/sagidayan/agent-cockpit/internal/agents"
	"github.com/sagidayan/agent-cockpit/internal/control"
	"github.com/sagidayan/agent-cockpit/internal/favorites"
	"github.com/sagidayan/agent-cockpit/internal/paths"
	"github.com/sagidayan/agent-cockpit/internal/snapshot"
)

func fav(args []string) error {
	switch {
	case len(args) == 0:
		return listFavs()
	case args[0] == "resume" && len(args) == 2:
		return resumeFav(args[1])
	case args[0] == "rm" && len(args) == 2:
		return removeFav(args[1])
	case len(args) == 1 && args[0] != "resume" && args[0] != "rm":
		return toggleFav(args[0])
	}
	return errUsage
}

func listFavs() error {
	list, err := favorites.Load()
	if err != nil {
		return err
	}
	live, err := agents.List()
	if err != nil {
		return err
	}
	fmt.Print(favTable(list, live))
	return nil
}

func favTable(list []snapshot.Entry, live []agents.Live) string {
	var rows [][]string
	for i, e := range list {
		status := "resumable"
		if favorites.Running(e, live) {
			status = "running"
		}
		rows = append(rows, []string{strconv.Itoa(i + 1), status, e.Kind, entryName(e), paths.Contract(e.Dir), children(e)})
	}
	return render(rows)
}

func children(e snapshot.Entry) string {
	if len(e.Children) == 0 {
		return ""
	}
	return fmt.Sprintf("+%d children", len(e.Children))
}

func entryName(e snapshot.Entry) string {
	return cmp.Or(e.Label, e.Name)
}

func toggleFav(target string) error {
	a, err := control.Find(target)
	if err != nil {
		return err
	}
	live, err := agents.List()
	if err != nil {
		return err
	}
	added, err := favorites.Toggle(a, live)
	if err != nil {
		return err
	}
	verb := "removed"
	if added {
		verb = "added"
	}
	fmt.Println(verb, cmp.Or(a.Label, a.WindowName))
	return nil
}

func resumeFav(key string) error {
	list, err := favorites.Load()
	if err != nil {
		return err
	}
	e, err := pick(list, key)
	if err != nil {
		return err
	}
	live, err := agents.List()
	if err != nil {
		return err
	}
	if favorites.Running(e, live) {
		return fmt.Errorf("%s is already running", entryName(e))
	}
	return resume(e, live)
}

func removeFav(key string) error {
	list, err := favorites.Load()
	if err != nil {
		return err
	}
	e, err := pick(list, key)
	if err != nil {
		return err
	}
	return favorites.Remove(e)
}

func pick(list []snapshot.Entry, key string) (snapshot.Entry, error) {
	if n, err := strconv.Atoi(key); err == nil {
		if n < 1 || n > len(list) {
			return snapshot.Entry{}, fmt.Errorf("no favorite #%d", n)
		}
		return list[n-1], nil
	}
	for _, field := range []func(snapshot.Entry) string{
		func(e snapshot.Entry) string { return e.Label },
		func(e snapshot.Entry) string { return e.Name },
	} {
		var found []snapshot.Entry
		for _, e := range list {
			if strings.EqualFold(field(e), key) {
				found = append(found, e)
			}
		}
		if len(found) == 1 {
			return found[0], nil
		}
		if len(found) > 1 {
			return snapshot.Entry{}, fmt.Errorf("%q matches %d favorites, use its number", key, len(found))
		}
	}
	return snapshot.Entry{}, errors.New("no favorite matches " + strconv.Quote(key))
}
