package com.rabbitmq.client.impl;

import com.netcracker.profiler.agent.Profiler;

import java.nio.ByteBuffer;

public class ChannelN {
    /** Records the publish, and the first 100 bytes of the body where the caller supplied one. */
    public void basicPublish$profiler(String exchange, String routingKey, byte[] body, Throwable throwable) {
        Profiler.event(exchange, "rabbitmq.exchange");
        Profiler.event(routingKey, "rabbitmq.routingKey");
        if (body != null) {
            int length = Math.min(body.length, 100);
            Profiler.event(new String(body, 0, length), "rabbitmq.message");
        }
    }

    /**
     * Records the publish for the overload amqp-client 5.31.0 added, reading the body without moving
     * the caller's position.
     *
     * <p>The body is null where the caller published through the {@code byte[]} overload with a null
     * array: from 5.31.0 that overload wraps a non-null array and passes null on unchanged, so an
     * unguarded read here would throw {@code NullPointerException} inside a publish the client
     * accepts.</p>
     */
    public void basicPublish$profiler(String exchange, String routingKey, ByteBuffer body, Throwable throwable) {
        Profiler.event(exchange, "rabbitmq.exchange");
        Profiler.event(routingKey, "rabbitmq.routingKey");
        if (body != null) {
            ByteBuffer duplicate = body.duplicate();
            byte[] bytes = new byte[Math.min(duplicate.remaining(), 100)];
            duplicate.get(bytes);
            Profiler.event(new String(bytes), "rabbitmq.message");
        }
    }
}
