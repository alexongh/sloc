package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

func main() {

    start := time.Now()

    args := os.Args; if len(args) != 2 {
        fmt.Println("Not exactly 1 argument provided")
        os.Exit(1)
    }

    targetDir := args[1]

    // Assure provided directory is valid
    err := os.Chdir(targetDir); if err != nil {
        fmt.Printf("Failed to chdir to %s. Error: %s", targetDir, err.Error())
        os.Exit(1)
    }

    // This is a bottleneck
    // Finding all dirs and files happen through recursion, no goroutines
    // Also everything has to be kept in memory
    result, err := crawl("."); if err != nil {
        fmt.Printf("Failed to crawl directory. error: %s", err.Error())
        os.Exit(1)
    }

    dirLen := len(result.Dirs)
    fileLen := len(result.Files)

    if len(result.Files) == 0 {
        fmt.Printf("No files found")
        os.Exit(1)
    }

    chunkSize := 100000

    eg, ctx := errgroup.WithContext(context.Background())
    
    iterations := len(result.Files) / chunkSize
    if iterations == 0 {
        iterations = 1
    }

    offset := 0
    totalLoc := 0
    mu := sync.Mutex{}

    for i := 0; i < iterations; i++ {

        offset += chunkSize
        end := offset+chunkSize

        if end > len(result.Files) {
            end = len(result.Files)
        }

        fileCunk := result.Files[i:end]

        eg.Go(func() error {
            totalLinesOfCode := 0
            for _, fileName := range fileCunk {
               
                if ctx.Err() != nil {
                    return ctx.Err()
                }

                lc, err := countLines(fileName); if err != nil {
                    return err 
                }

                totalLinesOfCode += lc
            }

            mu.Lock()
            totalLoc += totalLinesOfCode 
            mu.Unlock()

            return nil            
        })
    }

    err = eg.Wait(); if err != nil {
        fmt.Println(err.Error())
        os.Exit(1)
    }

    fmt.Print(fmt.Sprintf(`
Target directory:               %s
Iterations:                     %d
Chunk Size:                     %d
Total number of directories:    %d
Total number of files:          %d
Total lines of code:            %d
Total time spent:               %s
`, 
    targetDir,
    iterations,
    chunkSize,
    dirLen,
    fileLen,
    totalLoc,
    time.Since(start),
))
}

type Result struct {
    Dirs []string
    Files []string
}

func crawl(dir string) (Result, error) {

    defer func() {
        err := os.Chdir(".."); if err != nil {
            panic(err) 
        }
    }()

    err := os.Chdir(dir); if err != nil {
        return Result{}, err
    }

    wd, err := os.Getwd(); if err != nil {
        return Result{}, err
    }

    entries, err := os.ReadDir(wd); if err != nil {
        return Result{}, err
    }

    result := Result{
        Dirs: []string{},
        Files: []string{},
    }


    for _, entry := range entries {
        name := entry.Name()

        path := fmt.Sprintf("%s/%s", wd, name)

        // Skip hidden files and directories
        // Ignore windows for now
        // 46 for dot
        if name[0] == 46 {
            continue
        }

        if entry.IsDir() {
            result.Dirs = append(result.Dirs, path)
            continue
        }

        result.Files = append(result.Files, path)
    }

    if len(result.Dirs) > 0 {
        for _, dir := range result.Dirs {
            dirResult, err := crawl(dir); if err != nil {
                return Result{}, err 
            }

            result.Dirs = append(result.Dirs, dirResult.Dirs...)
            result.Files = append(result.Files, dirResult.Files...)
        }
    }

    return result, nil
}

func countLines(fileName string) (int, error) {
    file, err := os.Open(fileName); if err != nil {
        return 0, err
    }
    defer file.Close()

    if file == nil {
        return 0, fmt.Errorf("File was nil") 
    }

    lc := 0
    newLine := []byte{'\n'}
    for {
        // 8192 seems to be the sweet spot 
        buf := make([]byte, 8192)

        _, err := file.Read(buf)
        if err == io.EOF {
            break;
        }
        if err != nil {
            return 0, err
        }

        c := bytes.Count(buf, newLine)
        lc += c
    }

    return lc, nil
}

