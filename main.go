package main

import (
	"bufio"
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
	groupId, artifactId, version, name, path string
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
		for _, file := range files {
			fmt.Println(file)
			sp := strings.Split(file, "\\")
			if len(sp) != 5 {
				continue
			}
			if len(filterGroup) >= 1 && filterGroup[0] != "" {
				for _, prefix := range filterGroup {
					if !strings.HasPrefix(sp[0], prefix) {
						continue
					} else {
						var jf jarField
						jf.groupId = sp[0]
						jf.artifactId = sp[1]
						jf.version = sp[2]
						jf.name = sp[4]
						jf.path = file
						jfs = append(jfs, jf)
						if !slices.Contains(artifactIds, jf.artifactId) {
							if useCurl {
								// use curl (can not work sometimes)
								curlStr := "curl " + sfMavenUrl +
									"//" + jf.groupId +
									"//" + jf.artifactId +
									"//" + jf.version +
									"//" + jf.name +
									" --upload-file " + jf.path +
									" -k -u " + sfLogin + ":" + sfPass +
									" --request PUT"
								w.WriteString(curlStr + "\n")
							} else {
								artifactIds = append(artifactIds, jf.artifactId)
							}
						}
					}
				}
			} else {
				var jf jarField
				jf.groupId = sp[0]
				jf.artifactId = sp[1]
				jf.version = sp[2]
				jf.name = sp[4]
				jf.path = file
				jfs = append(jfs, jf)
				if !slices.Contains(artifactIds, jf.artifactId) {
					if useCurl {
						// use curl (can not work sometimes)
						curlStr := "curl " + sfMavenUrl +
							"//" + jf.groupId +
							"//" + jf.artifactId +
							"//" + jf.version +
							"//" + jf.name +
							" --upload-file " + jf.path +
							" -k -u " + sfLogin + ":" + sfPass +
							" --request PUT"
						w.WriteString(curlStr + "\n")
					} else {
						artifactIds = append(artifactIds, jf.artifactId)
					}
				}
			}
		}
		if !useCurl {
			// use maven deploy
			for _, art := range artifactIds {
				grouped := lo.GroupBy(jfs, func(index jarField) bool {
					return index.artifactId == art
				})
				versions := make([]string, 0)
				for i, arts := range grouped {
					if i {
						for _, ar := range arts {
							if !slices.Contains(versions, ar.version) {
								versions = append(versions, ar.version)
							}
						}
					} else {
						continue
					}
				}
				for _, ver := range versions {
					for i, arts := range grouped {
						groupedVer := lo.GroupBy(arts, func(index jarField) bool {
							return index.version == ver
						})
						if !i {
							continue
						}
						for u, byVer := range groupedVer {
							if !u {
								continue
							}
							if len(byVer) == 1 {
								mvnStr := "mvn" + " deploy:deploy-file" +
									" -DrepositoryId=" + mvnRepoId +
									" -DgroupId=" + byVer[0].groupId +
									" -DartifactId=" + byVer[0].artifactId +
									" -Dversion=" + byVer[0].version +
									" -Durl=" + sfMavenUrl +
									" -Dfile=" + byVer[0].path +
									" -DpomFile=" + byVer[0].path +
									" -s settings.xml"
								w.WriteString(mvnStr + "\n")
								continue
							}
							if len(byVer) == 2 {
								var pomFile string
								var jarFile string
								for _, ar := range byVer {
									if strings.HasSuffix(ar.path, ".jar") {
										// filteredJars = append(filteredJars, ar)
										jarFile = ar.path
									} else {
										pomFile = ar.path
									}
								}
								mvnStr := "mvn" + " deploy:deploy-file" +
									" -DrepositoryId=" + mvnRepoId +
									" -DgroupId=" + byVer[0].groupId +
									" -DartifactId=" + byVer[0].artifactId +
									" -Dversion=" + byVer[0].version +
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
		if name != ".git" && name != ".idea" && !info.IsDir() && (strings.HasSuffix(name, ".pom") || strings.HasSuffix(name, ".jar")) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
