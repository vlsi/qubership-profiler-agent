package com.netcracker.profilerTest.testapp;

import java.util.concurrent.TimeUnit;

public class Main {
    public static String test(String test) {
        // add non-trivial logic, so profiler instruments the method
        StringBuilder sb = new StringBuilder();
        for (int i = 0; i < test.length(); i++) {
            sb.append(test.charAt(i) | 0x20);
        }
        sb.setLength(0);
        for (int i = 0; i < test.length(); i++) {
            sb.append(test.charAt(i));
        }
        sb.setLength(0);
        for (int i = 0; i < test.length(); i++) {
            sb.append((char)(test.charAt(i) | 0x20));
        }
        return sb.toString();
    }

    public static void main(String[] args) throws InterruptedException {
        for (int i = 0; i < 5000; i++) {
            test("Hello, world!");
        }
        System.out.println(test("Hello, world!"));
        System.out.println("Waiting for profiler agent to initialize and send data...");
        // Give profiler agent time to initialize and connect to collector
        int delay = 5;
        if (args.length > 0) {
            try {
                delay = Integer.parseInt(args[0]);
            } catch (NumberFormatException e) {
                throw new RuntimeException("The first argument should be a delay in seconds", e);
            }
        }
        Thread.sleep(TimeUnit.SECONDS.toMillis(delay));
    }
}
