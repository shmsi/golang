package main

import (
	"cmp"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type NotFoundError struct {
	ID int
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("Task id %d not found", e.ID)
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

func (t *Task) String() string {
	mark := " "
	if t.IsDone {
		mark = "x"
	}
	return fmt.Sprintf("%3d [%s] %s", t.Id, mark, t.Title)
}

type TaskList []Task

func (l *TaskList) Add(title string) Task {
	largestID := 0
	for _, t := range *l {
		if t.Id > largestID {
			largestID = t.Id
		}
	}
	newTask := Task{Id: largestID + 1, Title: title, IsDone: false}
	*l = append(*l, newTask)
	return newTask
}

func (l *TaskList) Complete(id int) (Task, error) {
	var task Task
	found := false
	for i := range len(*l) {
		if (*l)[i].Id == id {
			(*l)[i].IsDone = true
			task = (*l)[i]
			found = true
			break
		}
	}
	var err error
	if !found {
		err = NotFoundError{ID: id}
	}
	return task, err
}

func (l *TaskList) Remove(id int) error {
	before := len(*l)
	*l = slices.DeleteFunc(*l, func(a Task) bool {
		return id == a.Id
	})
	if len(*l) == before {
		return NotFoundError{ID: id}
	}

	return nil
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

func load(path string) (TaskList, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var tasks TaskList
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)

	}
	return tasks, nil
}

func save(path string, tasks TaskList) error {
	// 1. create temp file
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}

	//2. Make sure it gets cleaned up if anything below fails
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding tasks: %w", err)
	}
	data = append(data, '\n')

	_, err = tmp.Write(data)
	if err != nil {
		return err
	}

	err = tmp.Sync()
	if err != nil {
		return err
	}

	if err := tmp.Close(); err != nil { // the real, checked close
		return err
	}
	return os.Rename(tmp.Name(), path)
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
		return err
	}
	task := tasks.Add(title)
	fmt.Printf("Added task with ID %d", task.Id)

	err = save(file, tasks)
	return err
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
		fmt.Println(task)

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
		return err
	}
	task, err := tasks.Complete(id)
	if err == nil {
		fmt.Printf("Completed task with id %d ", task.Id)
	}
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
	err = tasks.Remove(id)
	if err == nil {
		save(file, tasks)
	}
	return err
}
