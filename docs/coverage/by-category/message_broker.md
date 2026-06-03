<!-- DO NOT EDIT — generated from docs/coverage/registry.json by 'go run ./tools/coverage gen' -->
# message_broker

**Total**: 44 records · **C/C++**: 3 · **C#**: 1 · **elixir**: 2 · **JS/TS**: 3 · **multi**: 21 · **python**: 6 · **ruby**: 6 · **rust**: 2

Back to [summary](../summary.md). Bucket: **Other**.



## Schedulers

| Language | Name | Consumer extraction | Status | Notes |
|---|---|---|---|---|
| [C#](../by-language/csharp.md) | [Hangfire RecurringJob (.NET scheduled jobs)](../detail/msg.hangfire-recurring.md) | 🟢 | 🟢 | |
| [JS/TS](../by-language/jsts.md) | [node-schedule (Node scheduled jobs)](../detail/msg.node-schedule.md) | 🟢 | 🟢 | |
| [python](../by-language/python.md) | [APScheduler (Python advanced scheduler)](../detail/msg.apscheduler.md) | 🟢 | 🟢 | |
| [ruby](../by-language/ruby.md) | [rufus-scheduler (Ruby in-process scheduler)](../detail/msg.rufus-scheduler.md) | 🟢 | 🟢 | |
| [ruby](../by-language/ruby.md) | [whenever (Ruby cron / config/schedule.rb)](../detail/msg.whenever.md) | 🟢 | 🟢 | |

## Task Queues

| Language | Name | Consumer extraction | Producer extraction | Topic attribution | Status | Notes |
|---|---|---|---|---|---|---|
| [JS/TS](../by-language/jsts.md) | [BullMQ / bull (Node task queue)](../detail/msg.bullmq.md) | ✅ | ✅ | ✅ | ✅ | |
| [python](../by-language/python.md) | [Celery (Python task queue)](../detail/msg.celery.md) | ✅ | ✅ | ✅ | ✅ | |
| [python](../by-language/python.md) | [Dramatiq (Python task queue)](../detail/msg.dramatiq.md) | ✅ | ✅ | — | ✅ | |
| [ruby](../by-language/ruby.md) | [Resque (Ruby task queue)](../detail/msg.resque.md) | 🟢 | 🟢 | 🟢 | 🟢 | |
| [ruby](../by-language/ruby.md) | [Sidekiq (Ruby task queue)](../detail/msg.sidekiq.md) | 🟢 | 🟢 | — | 🟢 | |

## Brokers

| Language | Name | Consumer extraction | Producer extraction | Topic attribution | Status | Notes |
|---|---|---|---|---|---|---|
| [C/C++](../by-language/c-cpp.md) | [MQTT (Paho C/C++ / Mosquitto)](../detail/lang.c-cpp.framework.mqtt.md) | 🟢 | 🟢 | 🟢 | 🟢 | |
| [C/C++](../by-language/c-cpp.md) | [ZeroMQ (libzmq/cppzmq)](../detail/lang.c-cpp.framework.zeromq.md) | 🟢 | 🟢 | 🟢 | 🟢 | |
| [C/C++](../by-language/c-cpp.md) | [librdkafka (C/C++ Kafka client)](../detail/lang.c-cpp.framework.librdkafka.md) | 🟢 | 🟢 | 🟢 | 🟢 | |
| [elixir](../by-language/elixir.md) | [Broadway (Elixir data pipelines)](../detail/lang.elixir.framework.broadway.md) | 🟢 | 🟢 | 🟢 | 🟢 | |
| [JS/TS](../by-language/jsts.md) | [ORM model lifecycle-hook → handler TRIGGERS (TypeORM, Sequelize, Mongoose)](../detail/msg.orm-lifecycle-hooks-jsts.md) | ✅ | ✅ | ✅ | ✅ | |
| [multi](../by-language/multi.md) | [AMQP (generic)](../detail/msg.broker.amqp.md) | 🟢 | 🟢 | 🟢 | 🟢 | |
| [multi](../by-language/multi.md) | [AWS EventBridge](../detail/msg.broker.eventbridge.md) | 🟢 | 🟢 | 🟢 | 🟢 | |
| [multi](../by-language/multi.md) | [AWS SNS](../detail/msg.broker.sns.md) | ✅ | ✅ | ✅ | ✅ | |
| [multi](../by-language/multi.md) | [AWS SQS](../detail/msg.broker.sqs.md) | ✅ | ✅ | ✅ | ✅ | |
| [multi](../by-language/multi.md) | [Apache Kafka](../detail/msg.broker.kafka.md) | ✅ | ✅ | ✅ | ✅ | |
| [multi](../by-language/multi.md) | [Apache Pulsar](../detail/msg.broker.pulsar.md) | 🟢 | 🟢 | 🟢 | 🟢 | |
| [multi](../by-language/multi.md) | [Azure Event Grid](../detail/msg.broker.eventgrid.md) | 🟢 | 🟢 | 🟢 | 🟢 | |
| [multi](../by-language/multi.md) | [Azure Service Bus / Event Hubs](../detail/msg.broker.azure-service-bus.md) | 🟢 | 🟢 | 🟢 | 🟢 | |
| [multi](../by-language/multi.md) | [CloudEvents](../detail/msg.broker.cloudevents.md) | 🟢 | 🟢 | 🟢 | 🟢 | |
| [multi](../by-language/multi.md) | [Debezium (CDC)](../detail/msg.broker.debezium.md) | ✅ | ✅ | ✅ | ✅ | |
| [multi](../by-language/multi.md) | [GCP Pub/Sub](../detail/msg.broker.gcp-pubsub.md) | 🟢 | 🟢 | 🟢 | 🟢 | |
| [multi](../by-language/multi.md) | [Kafka Streams / Faust](../detail/msg.kafka-streams.md) | 🔴 | 🔴 | — | 🔴 | |
| [multi](../by-language/multi.md) | [MQTT](../detail/msg.broker.mqtt.md) | 🟢 | 🟢 | 🟢 | 🟢 | |
| [multi](../by-language/multi.md) | [NATS](../detail/msg.broker.nats.md) | ✅ | ✅ | ✅ | ✅ | |
| [multi](../by-language/multi.md) | [RabbitMQ](../detail/msg.broker.rabbitmq.md) | ✅ | ✅ | ✅ | ✅ | |
| [multi](../by-language/multi.md) | [Redis pub/sub & streams](../detail/msg.broker.redis.md) | ✅ | ✅ | ✅ | ✅ | |
| [python](../by-language/python.md) | [Django signals (intra-repo pub/sub)](../detail/msg.django-signals.md) | ✅ | ✅ | ✅ | ✅ | |
| [python](../by-language/python.md) | [ORM model lifecycle-hook → handler TRIGGERS (Django signals, SQLAlchemy events)](../detail/msg.orm-lifecycle-hooks-py.md) | ✅ | ✅ | ✅ | ✅ | |
| [ruby](../by-language/ruby.md) | [ORM model lifecycle-hook → handler TRIGGERS (ActiveRecord callbacks)](../detail/msg.orm-lifecycle-hooks-ruby.md) | ✅ | ✅ | ✅ | ✅ | |
| [rust](../by-language/rust.md) | [lapin (AMQP/RabbitMQ)](../detail/lang.rust.framework.lapin.md) | 🟢 | 🟢 | 🟢 | 🟢 | |
| [rust](../by-language/rust.md) | [rdkafka (Kafka)](../detail/lang.rust.framework.rdkafka.md) | 🟢 | 🟢 | 🟢 | 🟢 | |

## Realtime Channels

| Language | Name | Consumer extraction | Producer extraction | Room channel grouping | Topic attribution | Status | Notes |
|---|---|---|---|---|---|---|---|
| [elixir](../by-language/elixir.md) | [Phoenix Channels](../detail/msg.phoenix-channels.md) | 🔴 | ✅ | ✅ | 🟢 | 🔴 | |
| [multi](../by-language/multi.md) | [GraphQL subscriptions](../detail/msg.graphql-subscriptions.md) | ✅ | ✅ | — | ✅ | ✅ | |
| [multi](../by-language/multi.md) | [Server-Sent Events](../detail/msg.sse.md) | ✅ | ✅ | — | — | ✅ | |
| [multi](../by-language/multi.md) | [SignalR](../detail/msg.signalr.md) | 🔴 | ✅ | — | — | 🔴 | |
| [multi](../by-language/multi.md) | [WebSocket](../detail/msg.websocket.md) | ✅ | ✅ | ✅ | 🟢 | 🟢 | |
| [python](../by-language/python.md) | [Django Channels](../detail/msg.django-channels.md) | — | — | ✅ | — | ✅ | |
| [ruby](../by-language/ruby.md) | [Rails ActionCable](../detail/msg.actioncable.md) | — | — | ✅ | — | ✅ | |

## Webhooks

| Language | Name | Consumer extraction | Producer extraction | Signature verification | Topic attribution | Status | Notes |
|---|---|---|---|---|---|---|---|
| [multi](../by-language/multi.md) | [Webhooks](../detail/msg.webhook.md) | ✅ | ✅ | ✅ | 🟢 | 🟢 | |
