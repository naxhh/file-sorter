package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/naxhh/file-sorter/wp"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

func createJobs(rootFolder string) []wp.Job {
	files, err := ioutil.ReadDir(rootFolder)

	if err != nil {
		return []wp.Job{}
	}

	jobs := []wp.Job{}

	format1, _ := regexp.Compile("IMG_([0-9]{4})([0-9]{2})([0-9]{2}).*")
	format2, _ := regexp.Compile("([0-9]{4})([0-9]{2})([0-9]{2}).*")
	format3, _ := regexp.Compile("([0-9]{4})-([0-9]{2})-([0-9]{2}).*")

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

				if len(r) == 0 {
					r = format2.FindStringSubmatch(fileName)

					if len(r) == 0 {
						r = format3.FindStringSubmatch(fileName)

						if len(r) == 0 {
							return nil, moveToOthers(rootFolder, fileName)
						}
					}
				}

				yearInt, _ := strconv.Atoi(r[1])
				monthInt, _ := strconv.Atoi(r[2])
				dayInt, _ := strconv.Atoi(r[3])
				currentYear, _, _ := time.Now().Date()

				// if "too old" or too new assume is not a date
				if yearInt < currentYear-80 || yearInt > currentYear || monthInt < 1 || monthInt > 12 || dayInt < 1 || dayInt > 31 {
					return nil, moveToOthers(rootFolder, fileName)
				}

				year := r[1]
				month := r[2]

				if err := createFolders(year, month, rootFolder); err != nil {
					return nil, err
				}

				if err := moveFile(filepath.Join(rootFolder, fileName), filepath.Join(rootFolder, year, month, fileName)); err != nil {
					return nil, err
				}

				return nil, nil
			},
			Args: file.Name(),
		})
	}

	return jobs
}

func createJobsFixFolders(rootFolder string) []wp.Job {
	jobs := []wp.Job{}

	format, _ := regexp.Compile(".*/([0-9]{4})/([0-9]{2})/([0-9]{2})/.*")

	if os.PathSeparator != '/' {
		format, _ = regexp.Compile(".*\\([0-9]{4})\\([0-9]{2})\\([0-9]{2})\\.*")
	}

	err := filepath.Walk(rootFolder,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			jobs = append(jobs, wp.Job{
				Descriptor: wp.JobDescriptor{ID: wp.JobID(path), JType: "move", Metadata: nil},
				ExecFn: func(ctx context.Context, args interface{}) (interface{}, error) {
					path := args.(string)

					// if is year/month/day move file
					r := format.FindStringSubmatch(path)

					if len(r) == 0 {
						return nil, nil
					}

					year := r[1]
					month := r[2]


					_, fileName := filepath.Split(path)
					moveFile(path, filepath.Join(rootFolder, year, month, fileName))

					return nil, nil
				},
				Args: path,
			})

			return nil
		})
	if err != nil {
		fmt.Println(err)
	}

	return jobs
}

func createJobsDeleteEmptyFolders(rootFolder string) []wp.Job {
	jobs := []wp.Job{}

	err := filepath.Walk(rootFolder,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				return nil
			}

			jobs = append(jobs, wp.Job{
				Descriptor: wp.JobDescriptor{ID: wp.JobID(path), JType: "deleteFolder", Metadata: nil},
				ExecFn: func(ctx context.Context, args interface{}) (interface{}, error) {
					path := args.(string)

					// if has file insides do nothing
					if isEmpty, err := IsDirEmpty(path); err == nil && isEmpty {
						os.Remove(path)
					}

					return nil, nil
				},
				Args: path,
			})

			return nil
		})
	if err != nil {
		fmt.Println(err)
	}

	return jobs
}

func IsDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// read in ONLY one file
	_, err = f.Readdir(1)

	// and if the file is EOF... well, the dir is empty.
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

func createFolders(year string, month string, rootFolder string) error {
	path := filepath.Join(rootFolder, year)

	if err := createFolder(path); err != nil {
		return err
	}

	path = filepath.Join(path, month)

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

func moveToOthers(rootFolder string, fileName string) error {
	destFolder := filepath.Join(rootFolder, "others")
	if err := createFolder(destFolder); err != nil {
		return err
	}
	if err := moveFile(filepath.Join(rootFolder, fileName), filepath.Join(destFolder, fileName)); err != nil {
		return err
	}

	return nil
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println("file-sorter PATH [sort|deleteEmptyFolders|fixFolders]")
		return
	}

	rootFolder := os.Args[1]
	job := os.Args[2]

	fmt.Println("Sorting files from:", rootFolder)
	fmt.Println("Job:", job)

	workers := 10
	pool := wp.NewWorkerPool(workers)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	go func() {
		var jobs []wp.Job

		if job == "sort" {
			jobs = createJobs(rootFolder)
		} else if job == "deleteEmptyFolders" {
			jobs = createJobsDeleteEmptyFolders(rootFolder)
		} else if job == "fixFolders" {
			jobs = createJobsFixFolders(rootFolder)
		}

		pool.GenerateFrom(jobs)
	}()

	go pool.Run(ctx)

	for {
		select {
		case r, ok := <-pool.Results():
			if !ok {
				continue
			}

			if r.Err != nil {
				id := string(r.Descriptor.ID)

				fmt.Printf("failed Job '%s': %v\n", id, r.Err)
			}
		case <-pool.Done:
			fmt.Println("Job done")
			time.Sleep(10 * time.Second)
			return
		default:
		}
	}

}
