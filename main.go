package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"os"
	"os/exec"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/imports"

	// required so that the package is downloaded before functions get transformed
	_ "github.com/gin-gonic/gin"
)

var currPort int

func main() {
	numOfServices := 0
	currPort = 9000
	workingDir, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
	}
	filePath := "examples/test.go"
	outputDirPath := workingDir + "/output"
	writeToConsole := true
	writeToFiles := true
	writeDockerService := true

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, parser.AllErrors)
	if err != nil {
		fmt.Println(err)
	}

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Name.Name != "main" {
				numOfServices++
			}
		}
		return true
	})

	funcNames := make([]string, numOfServices)
	funcTypes := make([]*ast.FuncType, numOfServices)
	portNums := make([]int, numOfServices)
	services := make([]*ast.File, numOfServices)
	servicesFsets := make([]*token.FileSet, numOfServices)

	index := 0
	astutil.Apply(f, nil, func(c *astutil.Cursor) bool {
		n := c.Node()
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Name.Name != "main" {
				funcNames[index] = x.Name.Name
				funcTypes[index] = x.Type
				portNums[index] = currPort
				index++

				if x.Type.Params.NumFields() == 1 {
					if x.Type.Results.NumFields() == 1 {
						x.Body.List = oneArgFuncWithReturnBody(x.Type.Params.List[0].Names[0].Name, types.ExprString(x.Type.Results.List[0].Type))
					} else {
						x.Body.List = oneArgFuncBody(x.Type.Params.List[0].Names[0].Name)
					}
				} else {
					x.Body.List = simpleFuncBody()
				}
			}
		}

		return true
	})

	for i := 0; i < len(services); i++ {
		servicesFsets[i] = token.NewFileSet()
		services[i], err = parser.ParseFile(servicesFsets[0], filePath, nil, parser.AllErrors)
		if err != nil {
			fmt.Println(err)
		}
		astutil.Apply(services[i], nil, func(c *astutil.Cursor) bool {
			n := c.Node()
			switch x := n.(type) {
			case *ast.FuncDecl:
				if x.Name.Name != "main" && x.Name.Name != funcNames[i] {
					if x.Type.Params.NumFields() == 1 {
						if x.Type.Results.NumFields() == 1 {
							x.Body.List = oneArgFuncWithReturnBody(x.Type.Params.List[0].Names[0].Name, types.ExprString(x.Type.Results.List[0].Type))
						} else {
							x.Body.List = oneArgFuncBody(x.Type.Params.List[0].Names[0].Name)
						}
					} else {
						x.Body.List = simpleFuncBody()
					}
				} else if x.Name.Name == "main" {
					if funcTypes[i].Params.NumFields() == 1 {
						if funcTypes[i].Results.NumFields() == 1 {
							x.Body.List = oneArgWithReturnMainBody(funcNames[i], portNums[i], types.ExprString(funcTypes[i].Params.List[0].Type))
						} else {
							x.Body.List = oneArgMainBody(funcNames[i], portNums[i], types.ExprString(funcTypes[i].Params.List[0].Type))
						}
					} else {
						x.Body.List = simpleMainBody(funcNames[i], portNums[i])
					}
				}
			}
			return true
		})
		buf := new(bytes.Buffer)
		printer.Fprint(buf, servicesFsets[i], services[i])
		serviceSrc := buf.String()
		fmt.Println(serviceSrc)
		fixedBytes, err := imports.Process(filePath, []byte(serviceSrc), nil)
		if err != nil {
			fmt.Println(err)
		}

		if writeToConsole {
			fmt.Println("------------------ File " + funcNames[i] + " --------------------")
			fmt.Println(string(fixedBytes[:]))
		}
		if writeToFiles || writeDockerService {
			funcDir := outputDirPath + "/" + funcNames[i]
			os.MkdirAll(funcDir, 0755)
			os.WriteFile(funcDir+"/main.go", fixedBytes, 0644)
			os.Chdir(funcDir)
			err = exec.Command("go", "mod", "init", funcNames[i]).Run()
			if err != nil {
				fmt.Println(err)
			}
			err = exec.Command("go", "mod", "tidy").Run()
			if err != nil {
				fmt.Println(err)
			}
			if writeDockerService {
				os.WriteFile(funcDir+"/Dockerfile", []byte(getDockerFile(funcNames[i])), 0644)
			}
			if writeToFiles {
				err = exec.Command("go", "build").Run()
				if err != nil {
					fmt.Println(err)
				}
				os.WriteFile(outputDirPath+"/run.sh", []byte(getBashRunScript(funcNames)), 0755)
			}

			os.Chdir(workingDir)
		}
	}

	buf := new(bytes.Buffer)
	printer.Fprint(buf, fset, f)
	newSrc := buf.String()

	fixedBytes, err := imports.Process(filePath, []byte(newSrc), nil)
	if err != nil {
		fmt.Println(err)
	}
	if writeToConsole {
		fmt.Println("------------------ File Main --------------------")
		fmt.Println(string(fixedBytes[:]))
	}
	if writeToFiles || writeDockerService {
		funcDir := outputDirPath + "/main"
		os.MkdirAll(funcDir, 0755)
		os.WriteFile(funcDir+"/main.go", fixedBytes, 0644)
		os.Chdir(funcDir)
		err = exec.Command("go", "mod", "init", "main").Run()
		if err != nil {
			fmt.Println(err)
		}
		err = exec.Command("go", "mod", "tidy").Run()
		if err != nil {
			fmt.Println(err)
		}
		err = exec.Command("go", "build").Run()
		if err != nil {
			fmt.Println(err)
		}
		if writeDockerService {
			os.WriteFile(outputDirPath+"/docker-compose.yml", []byte(getDockerComposeYML(funcNames, portNums)), 0755)
		}
		if writeToFiles {
			os.WriteFile(outputDirPath+"/run.sh", []byte(getBashRunScript(funcNames)), 0755)
		}
		os.Chdir(workingDir)
	}
}

func simpleMainBody(funcName string, portNum int) []ast.Stmt {
	src := fmt.Sprintf(`
        package main

        import "fmt"
        import "net/http"
        import "github.com/gin-gonic/gin"

        func f() {
            r := gin.Default()
            r.GET("/", func(c *gin.Context) {
                %s()
                c.JSON(http.StatusOK, gin.H{})
            })
            err := r.Run(":%d")
            if err != nil {
                fmt.Println(err)
            }
        }
        `, funcName, portNum)
	return getBody(src)
}

func oneArgMainBody(funcName string, portNum int, argType string) []ast.Stmt {
	src := fmt.Sprintf(`
        package main

        import "fmt"
        import "net/http"
        import "encoding/json"
        import "github.com/gin-gonic/gin"

        func f() {
            r := gin.Default()
            r.POST("/", func(c *gin.Context) {
                jsonData, err := io.ReadAll(c.Request.Body)
                if err != nil {
                    fmt.Println(err)
                }
                var arg %s
                json.Unmarshal(jsonData, &arg)
                %s(arg)
                c.JSON(http.StatusOK, gin.H{})
            })
            err := r.Run(":%d")
            if err != nil {
                fmt.Println(err)
            }
        }
        `, argType, funcName, portNum)
	return getBody(src)
}

func oneArgWithReturnMainBody(funcName string, portNum int, argType string) []ast.Stmt {
	src := fmt.Sprintf(`
        package main

        import "fmt"
        import "net/http"
        import "encoding/json"
        import "github.com/gin-gonic/gin"

        func f() {
            r := gin.Default()
            r.POST("/", func(c *gin.Context) {
                jsonData, err := io.ReadAll(c.Request.Body)
                if err != nil {
                    fmt.Println(err)
                }
                var arg %s
                json.Unmarshal(jsonData, &arg)
                res := %s(arg)
                c.JSON(http.StatusOK, res)
            })
            err := r.Run(":%d")
            if err != nil {
                fmt.Println(err)
            }
        }
        `, argType, funcName, portNum)
	return getBody(src)
}

func simpleFuncBody() []ast.Stmt {
	src := fmt.Sprintf(`
        package main

        import "fmt"
        import "net/http"

        func f() {
            url := "http://localhost:%d"
            resp, err := http.Get(url)
            if err != nil {
                fmt.Println(err)
            }
            resp.Body.Close()
        }
        `, currPort)
	currPort++

	return getBody(src)
}

func oneArgFuncBody(argName string) []ast.Stmt {
	src := fmt.Sprintf(`
        package main

        import "fmt"
        import "net/http"
        import "encoding/json"
        import "bytes"

        func f(%s any) {
            url := "http://localhost:%d"
            jsonData, err := json.Marshal(%s)
            if err != nil {
                fmt.Println(err)
            }
            resp, err := http.Post(url, "aplication/json", bytes.NewReader(jsonData))
            if err != nil {
                fmt.Println(err)
            }
            resp.Body.Close()
        }
        `, argName, currPort, argName)
	currPort++

	return getBody(src)
}

func oneArgFuncWithReturnBody(argName string, retType string) []ast.Stmt {
	src := fmt.Sprintf(`
        package main

        import "fmt"
        import "net/http"
        import "encoding/json"
        import "bytes"

        func f(%s any) %s {
            url := "http://localhost:%d"
            jsonData, err := json.Marshal(%s)
            if err != nil {
                fmt.Println(err)
            }
            resp, err := http.Post(url, "aplication/json", bytes.NewReader(jsonData))
            if err != nil {
                fmt.Println(err)
            }
            body, err := io.ReadAll(resp.Body)
            if err != nil {
                fmt.Println(err)
            }
            var rtn %s 
            err = json.Unmarshal(body, &rtn)
            resp.Body.Close()
            return rtn
        }
        `, argName, retType, currPort, argName, retType)
	currPort++

	return getBody(src)
}

func getBody(src string) []ast.Stmt {
	var rtn []ast.Stmt
	f, err := parser.ParseFile(token.NewFileSet(), "", []byte(src), 0)
	if err != nil {
		fmt.Println(err)
	}

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			rtn = x.Body.List
		}
		return true
	})

	return rtn
}

func getBashRunScript(funcNames []string) string {
	functionsList := "./" + funcNames[0] + "/" + funcNames[0]
	for i := 1; i < len(funcNames); i++ {
		functionsList += "\" \"./" + funcNames[i] + "/" + funcNames[i]
	}

	script := `#!/bin/bash

# start the microservices as background processes
functions=("` + functionsList + `")
processes=()
for function in "${functions[@]}"; do
  eval "$function &"
  pid=$!
  processes+=("$pid")
done

echo "$processes"
sleep 5 # wait for servers to spin up

./main/main

# clean up the microservices
for process in "${processes[@]}"; do
  kill "$process"
done`

	return script
}

func getDockerFile(funcName string) string {
	return `FROM golang:alpine as builder
WORKDIR /src/service
COPY go.mod go.sum main.go ./
RUN go build -o ` + funcName + `
FROM alpine
WORKDIR /root/
COPY --from=builder /src/service ./service
CMD ["./service/` + funcName + `"]`
}

func getDockerComposeYML(funcNames []string, portNums []int) string {
	outfile := "services:\n"
	for i, funcName := range funcNames {
		outfile += fmt.Sprintf("  %s:\n", funcName)
		outfile += "    build:\n"
		outfile += fmt.Sprintf("      context: ./%s\n", funcName)
		outfile += fmt.Sprintf("      dockerfile: Dockerfile\n")
		outfile += "    ports:\n"
		outfile += fmt.Sprintf("      - \"%d:%d\"\n", portNums[i], portNums[i])
	}
	return outfile
}
