# sf-loader
## transfer your artefacts easy!

## Features

- Generate curl/mvn-deploy by local gradle cache
- Parse package-lock.json, download packages and generate curl to load

## Prepare .env file

You need to add .env file with this data:

- filterGroup=comma separated prefix to filter, empty for all (example 'org.junit,com.auth0')
- sfMavenUrl=url to maven repo
- sfNpmUrl=url to npm repo
- sfLogin=login
- sfPass=pass
- nexusLogin=login
- nexusPass=pass
- outputFile=name of result file (like deploy.cmd)
- useCurl=true/false (curl or mvn)
- mvnRepoId=repo id from maven settings.xml

## How to use

sf-loader requires [Go](https://go.dev/dl/) v1.21.5+ to build

Run command line in project directory with command

```sh
go build -o sf_loader.exe -ldflags "-s -w"
```

Then you need to build all gradle services, go to gradle cache dir (usually c:\Users\USER\.gradle\caches\modules-2\files-2.1\).
Put .env file and sf_loader.exe and run.

For npm packages put package-lock.json near the executable file, optionally in separated folder and run.

## License

MIT

**Enjoy!**