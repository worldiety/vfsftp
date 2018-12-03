package ftp

import (
	"github.com/jlaffaye/ftp"
	"github.com/worldiety/vfs"
	"io"
	"os"
)

type ftpDataProvider struct {
	conn *ftp.ServerConn
}

// Connect connects to the ftp and performs a login
func Connect(adr string, login string, password string) (vfs.DataProvider, error) {
	conn, err := ftp.Connect(adr)
	if err != nil {
		return nil, err
	}
	return &ftpDataProvider{conn}, nil
}

func (dp *ftpDataProvider) Read(path vfs.Path) (io.ReadCloser, error) {
	panic("implement me")
}

func (dp *ftpDataProvider) Write(path vfs.Path) (io.WriteCloser, error) {
	//dp.conn.Stor()
	panic("implement me")
}

func (dp *ftpDataProvider) Delete(path vfs.Path) error {
	panic("implement me")
}

func (dp *ftpDataProvider) ReadAttrs(path vfs.Path, dest interface{}) error {
	panic("implement me")
}

func (dp *ftpDataProvider) WriteAttrs(path vfs.Path, src interface{}) error {
	panic("implement me")
}

func (dp *ftpDataProvider) ReadDir(path vfs.Path) (vfs.DirEntList, error) {
	entries, err := dp.conn.List(path.String())
	if err != nil {
		return nil, err
	}

	return &fileInfoDirEntList{entries}, nil
}

func (dp *ftpDataProvider) MkDirs(path vfs.Path) error {
	panic("implement me")
}

func (dp *ftpDataProvider) Rename(oldPath vfs.Path, newPath vfs.Path) error {
	panic("implement me")
}

func (dp *ftpDataProvider) Close() error {
	panic("implement me")
}

type fileInfoDirEntList struct {
	list []*ftp.Entry
}

func (l *fileInfoDirEntList) ForEach(each func(scanner vfs.Scanner) error) error {
	scanner := &fileScanner{}
	for _, info := range l.list {
		scanner.info = info
		err := each(scanner)
		if err != nil {
			return err
		}
	}
	return nil
}

func (l *fileInfoDirEntList) Size() int64 {
	return int64(len(l.list))
}

//does nothing
func (l *fileInfoDirEntList) Close() error {
	return nil
}

//
type fileScanner struct {
	info *ftp.Entry
}

func (f *fileScanner) Scan(dest interface{}) error {
	if out, ok := dest.(*vfs.ResourceInfo); ok {
		out.Name = f.info.Name
		switch f.info.Type {
		case ftp.EntryTypeFile:
			out.Mode = 0
		case
			ftp.EntryTypeFolder:
			out.Mode = os.ModeDir
		case ftp.EntryTypeLink:
			out.Mode = os.ModeSymlink
		}
		out.Size = int64(f.info.Size)
		out.ModTime = f.info.Time.UnixNano() / 1e6
		return nil
	}
	return &vfs.UnsupportedAttributesError{Data: dest}
}
