# Go git searcher

Search all Git repositories in one or many directories and generate a CSV report with:

- project name
- path
- remote repository
- last commit date

## Run

```bash
go run main.go --directory="/path/to/directory";
go run main.go --directory="/path/to/directory1" --directory="/path/to/directory2";
```

## Tests

```bash
go test;
```
