package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"
)

type NotFoundError struct {
	ID int
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("Task id not found", e.ID)
}

var errUsage = errors.New("Usage error")

func usage() {

	fmt.Fprintln(os.Stderr, `usage: todo [-file path] <command> [args]

commands:
  add <title>      add a task
  list [-all]      list tasks (open ones only, unless -all)
  done <id>        mark a task as done
  remove <id>      delete a task`)
}

type Task struct {
	Id     int
	Title  string
	IsDone bool
}

func main() {
	file := flag.String("file", "todos.json", "where to store")
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() == 0 {
		usage()
		os.Exit(2)
	}

	cmd, args := flag.Arg(0), flag.Args()[1:]
	var err error
	switch cmd {
	case "add":
		err = runAdd(*file, args)
	case "list":
		err = runList(*file, args)
	case "done":
		err = runDone(*file, args)
	case "remove":
		err = runRemove(*file, args)
	default:
		err = fmt.Errorf("%w: unknown command %q", errUsage, cmd)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "todo:", err)
		if errors.Is(err, errUsage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

// func writeToFile(fileName string, line) error {

// 	f, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
// 	if err != nil {
// 		return err
// 	}
// 	defer f.Close()
// 	_, err = f.WriteString()
// }

func load(path string) ([]Task, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)

	}
	return tasks, nil
}

func save(path string, tasks []Task) error {
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding tasks: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

func runAdd(file string, args []string) error {
	fls := flag.NewFlagSet("add", flag.ExitOnError)
	err := fls.Parse(args)
	if err != nil {
		return fmt.Errorf("Could not parse flag")
	}

	title := strings.Join(fls.Args(), " ")
	if title == "" {
		return fmt.Errorf("%w: add needs a title", errUsage)
	}

	tasks, err := load(file)
	if err != nil {
		fmt.Errorf("Could not load tasks from file %s", file)
		return err
	}

	slices.SortFunc(tasks, func(a, b Task) int {
		return cmp.Compare(a.Id, b.Id)
	})
	largestID := 0
	if len(tasks) > 0 {
		largestID = tasks[len(tasks)-1].Id
	}
	newID := largestID + 1
	task := Task{Id: newID, Title: title, IsDone: false}

	tasks = append(tasks, task)
	err = save(file, tasks)
	if err != nil {
		fmt.Errorf("Could not add task %s to the list", title)
		return err
	}
	return nil
}

func runList(file string, args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	all := fs.Bool("all", false, "include completed tasks")
	fs.Parse(args)

	if fs.NArg() != 0 {
		return fmt.Errorf("%w: list takes no arguments", errUsage)
	}
	tasks, err := load(file)
	if err != nil {
		fmt.Errorf("Could not load tasks from file %s", file)
		return err
	}

	slices.SortFunc(tasks, func(a, b Task) int {
		return cmp.Compare(a.Id, b.Id)
	})

	fmt.Println(*all)

	for _, task := range tasks {
		if task.IsDone && !*all {
			continue
		}
		fmt.Printf("%3d     %s     %t\n", task.Id, task.Title, task.IsDone)

	}

	return nil
}

func parseID(args []string) (int, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("%w: expected exactly one task id", errUsage)
	}
	id, err := strconv.Atoi(args[0])
	if err != nil || id < 1 {
		return 0, fmt.Errorf("%w: invalid task id %q", errUsage, args[0])
	}
	return id, nil
}

func runDone(file string, args []string) error {
	fs := flag.NewFlagSet("done", flag.ExitOnError)
	fs.Parse(args)

	id, err := parseID(fs.Args())
	if err != nil {
		return err
	}

	tasks, err := load(file)
	if err != nil {
		fmt.Errorf("Could not load tasks from file %s", file)
		return err
	}

	found := false
	for i := range tasks {
		if tasks[i].Id == id {
			found = true
			tasks[i].IsDone = true
			break
		}
	}
	if !found {
		return NotFoundError{ID: id}
	}

	// for i := range tasks {
	// 	t := &tasks[i]
	// 	if t.Id == id {
	// 		t.IsDone = true
	// 	}
	// }
	err = save(file, tasks)
	return err
}

func runRemove(file string, args []string) error {
	fs := flag.NewFlagSet("remove", flag.ExitOnError)
	fs.Parse(args)
	id, err := parseID(fs.Args())
	if err != nil {
		return err
	}
	tasks, err := load(file)
	if err != nil {
		return err
	}
	before := len(tasks)
	tasks = slices.DeleteFunc(tasks, func(a Task) bool {
		return id == a.Id
	})
	if len(tasks) == before {
		return NotFoundError{ID: id}
	}
	err = save(file, tasks)
	return err
}
