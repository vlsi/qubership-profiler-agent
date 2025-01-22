package org.qubership.profiler.cli;

import net.sourceforge.argparse4j.inf.Namespace;
import org.testng.Assert;
import org.testng.annotations.Test;

import java.io.File;
import java.util.HashMap;
import java.util.Map;

public class ExportDumpTest {

    @Test
    public void dumpWalk() {
        File testDirectory = new File("target", "test-classes");
        Assert.assertTrue(testDirectory.isDirectory(), "test directory could not be found (expected 'target/test-classes')");
        File dumpDirectory = new File(testDirectory, "execution-statistics-collector");
        Assert.assertTrue(dumpDirectory.isDirectory(), "test dump directory not found (expected target/test-classes/execution-statistics-collector')");
        File outputFile = new File(testDirectory, "test_export.zip");
        if (outputFile.isFile()) {
            Assert.assertTrue(outputFile.delete(), "could not delete test export file - " + outputFile.getAbsolutePath());
        }

        Map<String, Object> map = new HashMap<>();
        map.put("start_date", "2022-02-02 20:21");
        map.put("end_date", "2022-02-02 20:22");
        map.put("time_zone", "Europe/Moscow");
        map.put("skip_details", false);
        map.put("dry_run", false);
        map.put("dump_root", dumpDirectory.getAbsolutePath());
        map.put("output_file", outputFile.getAbsolutePath());
        Namespace ns = new Namespace(map);
        ExportDump export = new ExportDump();
        int res = export.accept(ns);

        Assert.assertEquals(res, 0, "export.accept should be successful");
        Assert.assertTrue(outputFile.isFile(), "output file must be generated");
        Assert.assertTrue(outputFile.length() > 0, "output file must have non-zero length");
        Assert.assertTrue(outputFile.delete(), "could not delete test export file after test - " + outputFile.getAbsolutePath());
    }
}
