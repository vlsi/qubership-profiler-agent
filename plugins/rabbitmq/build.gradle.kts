plugins {
    id("build-logic.profiler-published-plugin")
    id("build-logic.test-junit5")
    id("build-logic.kotlin")
}

// ChannelN.basicPublish gained a seven-argument ByteBuffer overload in amqp-client 5.31.0, and the
// profiler binds an injected method by the exact descriptor of the call site, so each published body
// type needs its own basicPublish$profiler. 5.30.0 and 5.31.0 are the two sides of that boundary.
//
// spring-amqp 4.0 changed MessagingMessageListenerAdapter.invokeHandler to
// (Channel, org.springframework.messaging.Message, boolean, org.springframework.amqp.core.Message...)
// and moved InvocationResult to org.springframework.amqp.listener.adapter, so the rule in
// rabbitmq.xml matches no method there and the plugin records neither the consumer queue nor the
// connection URL. Until an injector for that signature exists, spring-rabbit is checked at the
// versions the rule does match.
instrumentationTestLibraries(
    "com.rabbitmq:amqp-client" to versions("5.30.0", "5.31.0"),
    "org.springframework.amqp:spring-rabbit" to versions("2.4.17", "3.2.6", trackLatest = false),
)
