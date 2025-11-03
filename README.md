# Go git searcher

Search all Git repositories in the directory(ies), and generate a `git_projects_report.csv` report file with the `project name`, `path`, `remote repository`, and `last commit date`. This will help you find like duplicate repositories with different last commit dates or something else.

## Run:

`go run main.go --directory="/path/to/directory";`\
`go run main.go --directory="/path/to/directory1" --directory="/path/to/directory2";`

## TODO

- unit tests