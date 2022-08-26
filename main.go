package main

import (
	"github.com/naxhh/file-sorter/wp"
	"context"
	"errors"
	"fmt"
	"os"
	 "path/filepath"
	"io/ioutil"
	"regexp"
)

func createJobs(rootFolder string) []wp.Job {
	files, err := ioutil.ReadDir(rootFolder)

    if err != nil {
        return []wp.Job{}
    }

    jobs := []wp.Job{}

    format1, _ := regexp.Compile("IMG_([0-9]{4})([0-9]{2})([0-9]{2}).*")
    format2, _ := regexp.Compile("([0-9]{4})([0-9]{2})([0-9]{2}).*")

    for _, file := range files {
    	if file.IsDir() {
    		continue
    	}

    	// TODO: this should really be channels to be fair...
    	jobs = append(jobs, wp.Job{
			Descriptor: wp.JobDescriptor{ID: wp.JobID(file.Name()), JType: "move", Metadata: nil},
			ExecFn: func(ctx context.Context, args interface{}) (interface{}, error) {
				fileName := args.(string)


				r := format1.FindStringSubmatch(fileName)

				if (len(r) == 0) {
					r = format2.FindStringSubmatch(fileName)

					if (len(r) == 0) {
						// TODO: put in "others" folder
						return nil, errors.New("Invalid file name format")
					}
				}

				year := r[1]
				month := r[2]
				day := r[3]

				if err := createFolders(year, month, day, rootFolder); err != nil {
					return nil, err
				}

				if err := moveFile(filepath.Join(rootFolder, fileName), filepath.Join(rootFolder, year, month, day, fileName)); err != nil {
					return nil, err
				}

				return nil, nil
			},
			Args: file.Name(),
		})
    }

	return jobs
}

func createFolders(year string, month string, day string, rootFolder string) error {
	path := filepath.Join(rootFolder, year)
	
	if err := createFolder(path); err != nil {
		return err
	}

	path = filepath.Join(path, month)

	if err := createFolder(path); err != nil {
		return err
	}

	path = filepath.Join(path, day)

	if err := createFolder(path); err != nil {
		return err
	}

	return nil
}

func createFolder(path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(path, os.ModePerm)
		if err != nil {
			return err
		}
	}

	return nil
}

func moveFile(origin string, destination string) error {
	return os.Rename(origin, destination)
}

func main() {
	rootFolder := os.Args[1]
	fmt.Println("Sorting files from:", rootFolder)

	workers := 10
	pool := wp.NewWorkerPool(workers)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	go pool.GenerateFrom(createJobs(rootFolder))
	go pool.Run(ctx)

	for {
		select {
		case r, ok := <-pool.Results():
			if !ok {
				continue
			}

			if (r.Err != nil) {
				id := string(r.Descriptor.ID)

				fmt.Printf("failed Job '%s': %v\n", id, r.Err)
			}

		case <-pool.Done:
			fmt.Println("Job done")
			return
		default:
		}
	}

}
