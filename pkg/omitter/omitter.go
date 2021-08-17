package omitter

// Omitter is the interface for a type which determines if a file should be included in the output
type Omitter interface {
	// File takes the filename and the path of the file and its return indicates if the file should be included.
	File(filename, path string) (bool, error)
	// Contents take the filepath and reads the file to determine if it should be retained or omitted.
	Contents(path string) (bool, error)
}
