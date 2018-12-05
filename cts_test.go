package vfsftp

import (
	"bytes"
	"fmt"
	. "github.com/worldiety/vfs"
	"strconv"
	"strings"
)

// A Check tells if a DataProvider has a specific property or not
type Check struct {
	Test        func(dp DataProvider) error
	Name        string
	Description string
}

// A CheckResult connects a Check and its execution result.
type CheckResult struct {
	Check  *Check
	Result error
}

type CTSResult []*CheckResult

// String returns a markdown representation of this result
func (c CTSResult) String() string {
	sb := &strings.Builder{}
	sb.WriteString("| CTS Check     | Result        |\n")
	sb.WriteString("| ------------- | ------------- |\n")
	for _, check := range c {
		sb.WriteString("| ")
		sb.WriteString(check.Check.Name)
		sb.WriteString("|")
		if check.Result != nil {
			sb.WriteString(":heavy_exclamation_mark:")
		} else {
			sb.WriteString(":white_check_mark: ")
		}
		sb.WriteString("|\n")
	}

	return sb.String()
}

type CTS struct {
	checks []*Check
}

func (t *CTS) All() {
	t.checks = []*Check{
		CheckIsEmpty,
		CheckCanWrite0,
		CheckReadAny,
		CheckWriteAndRead,
		CheckRename,
		UnsupportedAttributes,
		CloseProvider,
	}
}

func (t *CTS) Run(dp DataProvider) CTSResult {
	res := make([]*CheckResult, 0)
	for _, check := range t.checks {
		SetDefault(dp)
		err := check.Test(dp)
		res = append(res, &CheckResult{check, err})
	}
	return res
}

func generateTestSlice(len int) []byte {
	tmp := make([]byte, len)
	for i := 0; i < len; i++ {
		tmp[i] = byte(i)
	}
	return tmp
}

//======== our actual checks =============
var CheckIsEmpty = &Check{
	Test: func(dp DataProvider) error {
		list, err := ReadDirEnt("")
		if err != nil {
			return err
		}
		if len(list) == 0 {
			return nil
		}
		//not empty, try to clear to make test a bit more robust
		for _, entry := range list {
			err := dp.Delete(Path(entry.Name))
			if err != nil {
				return err
			}
		}
		// recheck
		list, err = ReadDirEnt("")
		if err != nil {
			return err
		}
		if len(list) == 0 {
			return nil
		}
		return fmt.Errorf("DataProvider is not empty and cannot clear it")
	},
	Name:        "Empty",
	Description: "Checks the corner case of an empty DataProvider",
}

var CheckCanWrite0 = &Check{
	Test: func(dp DataProvider) error {
		paths := []Path{"", "/", "/canWrite0", "/canWrite0/subfolder", "canWrite0_1/subfolder1/subfolder2"}
		lengths := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 512, 1024, 4096, 4097, 8192, 8193}
		for _, path := range paths {
			for _, testLen := range lengths {
				tmp := generateTestSlice(testLen)
				writer, err := dp.Write(path.Child(strconv.Itoa(testLen) + ".bin"))
				if err != nil {
					return err
				}
				n, err := writer.Write(tmp)
				if err != nil {
					writer.Close()
					return err
				}

				err = writer.Close()
				if err != nil {
					return err
				}

				if n != len(tmp) {
					return fmt.Errorf("expected to write %v bytes but just wrote %v", len(tmp), n)
				}
			}
		}

		return nil
	},
	Name:        "Write any",
	Description: "Write some simple files with various lengths in various paths",
}

var CheckReadAny = &Check{
	Test: func(dp DataProvider) error {
		list, err := ReadDirEntRecur("")
		if err != nil {
			return err
		}
		if len(list) == 0 {
			return fmt.Errorf("expected at least 1 file")
		}

		for _, entry := range list {
			if entry.Resource.Mode.IsDir() {
				continue
			}
			tmp, err := ReadAll(entry.Path)
			if err != nil {
				return err
			}
			if len(tmp) != int(entry.Resource.Size) {
				return fmt.Errorf("expected same size of %v: expected %v bytes but got %v", entry.Path, entry.Resource.Size, len(tmp))
			}
		}
		return nil
	},
	Name:        "Read any",
	Description: "Asserts that nothing is empty and everything can be read",
}

var CheckWriteAndRead = &Check{
	Test: func(dp DataProvider) error {
		paths := []Path{"", "/", "/canWrite1", "/canWrite1/subfolder", "canWrite1_1/subfolder1/subfolder2"}
		lengths := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 512, 1024, 4096, 4097, 8192, 8193}
		for _, path := range paths {
			for _, testLen := range lengths {
				tmp := generateTestSlice(testLen)
				child := path.Child(strconv.Itoa(testLen) + ".bin")
				writer, err := Write(child)
				if err != nil {
					return err
				}
				n, err := writer.Write(tmp)
				if err != nil {
					writer.Close()
					return err
				}

				err = writer.Close()
				if err != nil {
					return err
				}

				if n != len(tmp) {
					return fmt.Errorf("expected to write %v bytes but just wrote %v", len(tmp), n)
				}

				data, err := ReadAll(child)
				if err != nil {
					return err
				}

				if bytes.Compare(data, tmp) != 0 {
					return fmt.Errorf("expected that written and read bytes are equal but %v != %v", tmp, data)
				}
			}
		}

		return nil
	},
	Name:        "Write and Read",
	Description: "Write some stuff and read it agains",
}

var CheckRename = &Check{
	Test: func(dp DataProvider) error {
		a := Path("/a.bin")
		b := Path("/b.bin")

		err := dp.Delete(a)
		if err != nil {
			return err
		}

		err = Delete(b)
		if err != nil {
			return err
		}

		//renaming of non-a to non-b must fail
		err = Rename(a, b)
		if err == nil {
			return fmt.Errorf("renaming of non-a to non-b must fail")
		}

		test0 := generateTestSlice(7)
		_, err = WriteAll(a, test0)
		if err != nil {
			return err
		}

		// a exists and b not, must succeed
		err = dp.Rename(a, b)
		if err != nil {
			return err
		}
		_, err = Stat(a)
		if err == nil {
			return fmt.Errorf("a must be ResourceNotFound")
		}
		info, err := Stat(b)
		if err != nil {
			return fmt.Errorf("b must be available")
		}
		if info.Size != 7 {
			return fmt.Errorf("a must be 7 bytes long but is %v", info.Size)
		}

		// b exists and c exists, must succeed
		c := Path("/c.bin")
		_, err = WriteAll(c, generateTestSlice(13))
		if err != nil {
			return err
		}

		err = dp.Rename(b, c)
		if err != nil {
			return err
		}
		_, err = Stat(b)
		if err == nil {
			return fmt.Errorf("b must be ResourceNotFound")
		}

		info, err = Stat(c)
		if err != nil {
			return err
		}
		if info.Size != 7 {
			return fmt.Errorf("a must be 7 bytes long but is %v", info.Size)
		}
		return nil
	},
	Name:        "Rename",
	Description: "Renames and their corner cases",
}

var UnsupportedAttributes = &Check{Test: func(dp DataProvider) error {
	c := Path("/c.bin")
	_, err := WriteAll(c, generateTestSlice(13))
	if err != nil {
		return err
	}
	mustSupport := &ResourceInfo{}
	err = dp.ReadAttrs(c, mustSupport)
	if err != nil {
		return err
	}

	mustNotSupport := &unsupportedType{}
	err = dp.ReadAttrs(c, mustNotSupport)
	if err == nil {
		return fmt.Errorf("reading into a generic unsupportedType{} with private members and no public fields is an error")
	}
	if UnwrapUnsupportedAttributesError(err) == nil {
		return fmt.Errorf("expected UnsupportedAttributesError but got %v", err)
	}

	err = ReadAttrs(c, "hello world")
	if err == nil {
		return fmt.Errorf("reading into a value type like a string is always a programming error")
	}
	if UnwrapUnsupportedAttributesError(err) == nil {
		return fmt.Errorf("expected UnsupportedAttributesError but got %v", err)
	}

	dir, err := dp.ReadDir("")
	if err != nil {
		return err
	}

	dir, err = ReadDir("")
	if err != nil {
		return err
	}

	count := 0
	err = dir.ForEach(func(scanner Scanner) error {
		mustSupport := &ResourceInfo{}
		err = scanner.Scan(mustSupport)
		if err != nil {
			return err
		}

		mustNotSupport := &unsupportedType{}
		err = scanner.Scan(mustNotSupport)
		if err == nil {
			return fmt.Errorf("reading into a generic unsupportedType{} with private members and no public fields is an error")
		}
		if UnwrapUnsupportedAttributesError(err) == nil {
			return fmt.Errorf("expected UnsupportedAttributesError but got %v", err)
		}

		err = scanner.Scan("hello world")
		if err == nil {
			return fmt.Errorf("reading into a value type like a string is always a programming error")
		}
		if UnwrapUnsupportedAttributesError(err) == nil {
			return fmt.Errorf("expected UnsupportedAttributesError but got %v", err)
		}

		count++

		return nil
	})
	if err != nil {
		return err
	}
	if count <= 0 {
		return fmt.Errorf("expected at least 1 file to scan")
	}
	err = dir.Close()
	if err != nil {
		return err
	}

	// same for write
	err = dp.WriteAttrs(c, mustNotSupport)
	if err == nil {
		return fmt.Errorf("writing from a generic unsupportedType{} with private members and no public fields is an error")
	}
	if UnwrapUnsupportedAttributesError(err) == nil && UnwrapUnsupportedOperationError(err) == nil {
		return fmt.Errorf("expected UnsupportedAttributesError or UnsupportedOperationError but got %v", err)
	}

	return nil

},
	Name:        "Attributes",
	Description: "Tries to read unsupported attributes.",
}

type unsupportedType struct {
	atLeastHiddenFieldsAreNotAllowed string
}

var CloseProvider = &Check{
	Test: func(dp DataProvider) error {
		err := dp.Close()
		if err != nil {
			return err
		}

		return nil
	},
	Name:        "Close",
	Description: "Simply checks if close succeeds. It does not mean that the DataProvider is unusable, because some are stateless",
}
