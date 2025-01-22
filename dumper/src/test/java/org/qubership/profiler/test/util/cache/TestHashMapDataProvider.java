package org.qubership.profiler.test.util.cache;

import org.testng.annotations.DataProvider;

public class TestHashMapDataProvider {
    @DataProvider(name = "big-tests")
    public static Object[][] createInstances() {
        Object[][] tests = new Object[50][];
        for (int j = 0; j < 5; j++)
            for (int i = 0; i < 10; i++)
                tests[i + j * 10] = new Object[]{50 + i + j * 10};
        return tests;
    }
}
