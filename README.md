# sonar-build-breaker

install with:

```shell
go install github.com/shadowpriest/sonar-build-breaker@latest
```

use for CI  to break build when sonar check fails.

this program will automatic detect `report-task.txt` file 

examples:

```shell
mvn package sonar:sonar
sonar-build-breaker
# when sonar fail,deploy will not execute
mvn deploy -DskipTests
```



