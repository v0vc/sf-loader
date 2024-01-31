package main

import (
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/samber/lo"
)

type jarField struct {
	GroupID    string `xml:"groupId,omitempty"`
	ArtifactID string `xml:"artifactId,omitempty"`
	Version    string `xml:"version,omitempty"`
	Name       string
	Path       string
}

type jarFieldParent struct {
	Parent struct {
		ArtifactID string `xml:"artifactId,omitempty"`
		GroupID    string `xml:"groupId,omitempty"`
		Version    string `xml:"version,omitempty"`
	} `xml:"parent,omitempty"`
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	filterGroup := strings.Split(os.Getenv("filterGroup"), ",")
	sfMavenUrl := os.Getenv("sfMavenUrl")
	sfNpmUrl := os.Getenv("sfNpmUrl")
	sfLogin := os.Getenv("sfLogin")
	sfPass := os.Getenv("sfPass")
	nexusLogin := os.Getenv("nexusLogin")
	nexusPass := os.Getenv("nexusPass")
	outputFile := os.Getenv("outputFile")
	useCurl, _ := strconv.ParseBool(os.Getenv("useCurl"))
	mvnRepoId := os.Getenv("mvnRepoId")
	useGradleCache, _ := strconv.ParseBool(os.Getenv("useGradleCache"))

	curDir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("running in " + curDir)

	f, _ := os.Create(filepath.Join(curDir, outputFile))
	defer f.Close()
	w := bufio.NewWriter(f)

	npmFile, err := os.OpenFile(filepath.Join(curDir, "package-lock.json"), os.O_RDONLY, os.ModePerm)
	if errors.Is(err, os.ErrNotExist) {
		// считаем что запустили в папке с кэшом джавы
		files, er := filePathWalkDir("./")
		if er != nil {
			panic(er)
		}
		jfs := make([]jarField, 0)
		artifactIds := make([]string, 0)
		for i, file := range files {
			fmt.Println(file)
			// запустили в папке с гредл кешом
			if useGradleCache {
				sp := strings.Split(file, string(os.PathSeparator))
				if len(sp) != 5 {
					continue
				}
				if len(filterGroup) >= 1 && filterGroup[0] != "" {
					for _, prefix := range filterGroup {
						if !strings.HasPrefix(sp[0], prefix) {
							continue
						} else {
							var jf jarField
							jf.GroupID = sp[0]
							jf.ArtifactID = sp[1]
							jf.Version = sp[2]
							jf.Name = sp[4]
							jf.Path = file
							jfs = append(jfs, jf)
							if !slices.Contains(artifactIds, jf.ArtifactID) {
								if useCurl {
									// use curl (can not work sometimes)
									curlStr := "curl " + sfMavenUrl +
										"//" + jf.GroupID +
										"//" + jf.ArtifactID +
										"//" + jf.Version +
										"//" + jf.Name +
										" --upload-file " + jf.Path +
										" -k -u " + sfLogin + ":" + sfPass +
										" --request PUT"
									w.WriteString(curlStr + "\n")
								} else {
									artifactIds = append(artifactIds, jf.ArtifactID)
								}
							}
						}
					}
				} else {
					var jf jarField
					jf.GroupID = sp[0]
					jf.ArtifactID = sp[1]
					jf.Version = sp[2]
					jf.Name = sp[4]
					jf.Path = file
					jfs = append(jfs, jf)
					if !slices.Contains(artifactIds, jf.ArtifactID) {
						if useCurl {
							// use curl (can not work sometimes)
							curlStr := "curl " + sfMavenUrl +
								"//" + jf.GroupID +
								"//" + jf.ArtifactID +
								"//" + jf.Version +
								"//" + jf.Name +
								" --upload-file " + jf.Path +
								" -k -u " + sfLogin + ":" + sfPass +
								" --request PUT"
							w.WriteString(curlStr + "\n")
						} else {
							artifactIds = append(artifactIds, jf.ArtifactID)
						}
					}
				}
			} else {
				// запустили в папке с мавен кешом
				fileExtension := filepath.Ext(file)
				if fileExtension == ".pom" {
					pomField, ere := Parse(file)
					if ere != nil {
						continue
					}
					if pomField.GroupID == "" || pomField.Version == "" || pomField.ArtifactID == "" {
						pomParent, e := ParseParent(file)
						if e != nil {
							continue
						}
						if pomField.GroupID == "" && pomParent.Parent.GroupID != "" {
							pomField.GroupID = pomParent.Parent.GroupID
						}
						if pomField.Version == "" && pomParent.Parent.Version != "" {
							pomField.Version = pomParent.Parent.Version
						}
						if pomField.ArtifactID == "" && pomParent.Parent.ArtifactID != "" {
							pomField.ArtifactID = pomParent.Parent.ArtifactID
						}
					}
					pomField.Name = filepath.Base(file)
					pomField.Path = file
					if len(filterGroup) >= 1 && filterGroup[0] != "" {
						for _, prefix := range filterGroup {
							if !strings.HasPrefix(pomField.GroupID, prefix) {
								continue
							} else {
								jfs = append(jfs, *pomField)
								if !slices.Contains(artifactIds, pomField.ArtifactID) {
									if useCurl {
										// use curl (can not work sometimes)
										curlStr := "curl " + sfMavenUrl +
											"//" + pomField.GroupID +
											"//" + pomField.ArtifactID +
											"//" + pomField.Version +
											"//" + pomField.Name +
											" --upload-file " + pomField.Path +
											" -k -u " + sfLogin + ":" + sfPass +
											" --request PUT"
										w.WriteString(curlStr + "\n")
									} else {
										artifactIds = append(artifactIds, pomField.ArtifactID)
									}
								}
							}
						}
					} else {
						var jf jarField
						jf.GroupID = pomField.GroupID
						jf.ArtifactID = pomField.ArtifactID
						jf.Version = pomField.Version
						jf.Name = pomField.Name
						jf.Path = file
						jfs = append(jfs, jf)
						if !slices.Contains(artifactIds, jf.ArtifactID) {
							if useCurl {
								// use curl (can not work sometimes)
								curlStr := "curl " + sfMavenUrl +
									"//" + jf.GroupID +
									"//" + jf.ArtifactID +
									"//" + jf.Version +
									"//" + jf.Name +
									" --upload-file " + jf.Path +
									" -k -u " + sfLogin + ":" + sfPass +
									" --request PUT"
								w.WriteString(curlStr + "\n")
							} else {
								artifactIds = append(artifactIds, jf.ArtifactID)
							}
						}
					}
				} else {
					pomField, ere := Parse(files[i+1])
					if ere != nil {
						continue
					}
					if pomField.GroupID == "" || pomField.Version == "" || pomField.ArtifactID == "" {
						pomParent, e := ParseParent(files[i+1])
						if e != nil {
							continue
						}
						if pomField.GroupID == "" && pomParent.Parent.GroupID != "" {
							pomField.GroupID = pomParent.Parent.GroupID
						}
						if pomField.Version == "" && pomParent.Parent.Version != "" {
							pomField.Version = pomParent.Parent.Version
						}
						if pomField.ArtifactID == "" && pomParent.Parent.ArtifactID != "" {
							pomField.ArtifactID = pomParent.Parent.ArtifactID
						}
					}
					pomField.Name = filepath.Base(file)
					pomField.Path = file
					if len(filterGroup) >= 1 && filterGroup[0] != "" {
						for _, prefix := range filterGroup {
							if !strings.HasPrefix(pomField.GroupID, prefix) {
								continue
							} else {
								jfs = append(jfs, *pomField)
								if !slices.Contains(artifactIds, pomField.ArtifactID) {
									if useCurl {
										// use curl (can not work sometimes)
										curlStr := "curl " + sfMavenUrl +
											"//" + pomField.GroupID +
											"//" + pomField.ArtifactID +
											"//" + pomField.Version +
											"//" + pomField.Name +
											" --upload-file " + pomField.Path +
											" -k -u " + sfLogin + ":" + sfPass +
											" --request PUT"
										w.WriteString(curlStr + "\n")
									} else {
										artifactIds = append(artifactIds, pomField.ArtifactID)
									}
								}
							}
						}
					} else {
						var jf jarField
						jf.GroupID = pomField.GroupID
						jf.ArtifactID = pomField.ArtifactID
						jf.Version = pomField.Version
						jf.Name = pomField.Name
						jf.Path = file
						jfs = append(jfs, jf)
						if !slices.Contains(artifactIds, jf.ArtifactID) {
							if useCurl {
								// use curl (can not work sometimes)
								curlStr := "curl " + sfMavenUrl +
									"//" + jf.GroupID +
									"//" + jf.ArtifactID +
									"//" + jf.Version +
									"//" + jf.Name +
									" --upload-file " + jf.Path +
									" -k -u " + sfLogin + ":" + sfPass +
									" --request PUT"
								w.WriteString(curlStr + "\n")
							} else {
								artifactIds = append(artifactIds, jf.ArtifactID)
							}
						}
					}
				}
			}
		}
		if !useCurl {
			// use maven deploy
			for _, art := range artifactIds {
				grouped := lo.GroupBy(jfs, func(index jarField) bool {
					return index.ArtifactID == art
				})
				versions := make([]string, 0)
				for i, arts := range grouped {
					if i {
						for _, ar := range arts {
							if !slices.Contains(versions, ar.Version) {
								versions = append(versions, ar.Version)
							}
						}
					} else {
						continue
					}
				}
				for _, ver := range versions {
					for i, arts := range grouped {
						groupedVer := lo.GroupBy(arts, func(index jarField) bool {
							return index.Version == ver
						})
						if !i {
							continue
						}
						for u, byVer := range groupedVer {
							if !u {
								continue
							}
							if len(byVer) == 1 {
								mvnStr := "call mvn deploy:deploy-file" +
									" -Dmaven.wagon.http.ssl.insecure=true" +
									" -Dmaven.wagon.http.ssl.allowall=true" +
									" -Dmaven.wagon.http.ssl.ignore.validity.dates=true" +
									" -DrepositoryId=" + mvnRepoId +
									" -DgroupId=" + byVer[0].GroupID +
									" -DartifactId=" + byVer[0].ArtifactID +
									" -Dversion=" + byVer[0].Version +
									" -Durl=" + sfMavenUrl +
									" -Dfile=" + byVer[0].Path +
									" -DpomFile=" + byVer[0].Path +
									" -s settings.xml"
								w.WriteString(mvnStr + "\n")
								continue
							}
							if len(byVer) == 2 {
								var pomFile string
								var jarFile string
								for _, ar := range byVer {
									if strings.HasSuffix(ar.Path, ".jar") {
										jarFile = ar.Path
									} else {
										pomFile = ar.Path
									}
								}
								mvnStr := "call mvn deploy:deploy-file" +
									" -Dmaven.wagon.http.ssl.insecure=true" +
									" -Dmaven.wagon.http.ssl.allowall=true" +
									" -Dmaven.wagon.http.ssl.ignore.validity.dates=true" +
									" -DrepositoryId=" + mvnRepoId +
									" -DgroupId=" + byVer[0].GroupID +
									" -DartifactId=" + byVer[0].ArtifactID +
									" -Dversion=" + byVer[0].Version +
									" -Durl=" + sfMavenUrl +
									" -Dfile=" + jarFile +
									" -DpomFile=" + pomFile +
									" -s settings.xml"
								w.WriteString(mvnStr + "\n")
								continue
							}
						}
					}
				}
			}
		}
	} else {
		// работаем по package-lock.json
		rd := bufio.NewReader(npmFile)
		var res []string
		for {
			line, er := rd.ReadString('\n')
			if er != nil {
				if er == io.EOF {
					break
				}
				log.Fatalf("read file line error: %v", er)
			}

			rowQ := strings.TrimSpace(line)
			row := strings.Trim(rowQ, "\"")
			if strings.HasPrefix(row, "resolved") {
				tag := strings.Split(row, ":")
				if len(tag) == 3 {
					url := tag[1] + ":" + strings.Trim(tag[2], ",")
					fns := strings.Split(url, "/")
					fn := fns[len(fns)-1]
					res = append(res, strings.Trim(fn, "\""))
					curlStr := "curl" + " -k -u " + nexusLogin + ":" + nexusPass + " -O" + url
					w.WriteString(curlStr + "\n")
				}
			}
		}
		for _, tgz := range res {
			curlStr := "curl" + " -k -u " + sfLogin + ":" + sfPass + " -F " + "npm.asset=@" + tgz + " " + sfNpmUrl
			w.WriteString(curlStr + "\n")
		}
	}
	w.WriteString("pause" + "\n")
	w.Flush()
}

func filePathWalkDir(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		name := info.Name()
		if info.IsDir() && (info.Name() == ".git" || info.Name() == ".idea" || info.Name() == "__MACOSX") {
			return filepath.SkipDir
		} else if strings.HasSuffix(name, ".pom") || strings.HasSuffix(name, ".jar") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func Parse(path string) (*jarField, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, _ := io.ReadAll(file)
	var project jarField

	err = xml.Unmarshal(b, &project)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func ParseParent(path string) (*jarFieldParent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, _ := io.ReadAll(file)
	var project jarFieldParent

	err = xml.Unmarshal(b, &project)
	if err != nil {
		return nil, err
	}
	return &project, nil
}
