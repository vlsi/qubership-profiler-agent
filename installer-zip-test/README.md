Executes tests with `-javaagent:.../qubership-profiler-agent.jar`, so it enables to test the following:
* `installer.zip` contents: build system extracts the profiler from `installer.zip`
* configuration files and the jar files in `installer.zip`: the profiler activates with `-javaagent:..`, so the configuration and the classes load like they would do with regular applications
