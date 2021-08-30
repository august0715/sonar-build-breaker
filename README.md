# sonar-build-breaker

use for CI  to break build when sonar check fails

examples:

```
mvn package sonar:sonar
sonar-build-breaker
# when sonar fail,deploy will not execute
mvn deploy -DskipTests
```



this program will automatic detect report-task.txt 

