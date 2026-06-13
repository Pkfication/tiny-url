# TinyURL - Distributed Key Generation Service (KGS)

This repository contains the implementation and experimentation details for a highly scalable, distributed Key Generation Service (KGS) for a TinyURL clone. The service generates unique, collision-free short URLs (Base62 strings) and is designed to handle extreme concurrency.

## Architecture Overview

*   **Language**: Golang
*   **Coordination**: Apache ZooKeeper
*   **Load Balancing**: Nginx API Gateway
*   **Observability**: Elastic Stack (Elasticsearch, Kibana, APM Server)
*   **Containerization**: Docker & Docker Compose

### 1. Key Generation Strategy
To ensure absolutely no key collisions across horizontally scaled instances, the KGS uses **Apache ZooKeeper**. 
Instead of hitting ZooKeeper for every single key request, each KGS instance claims a large, mutually exclusive "block" or "range" of keys at a time. The instance dispenses these keys from local memory. Once an instance exhausts its range, it goes back to ZooKeeper to fetch a new block.

### 2. High-Performance Load Balancing & Autoscaling
We use an **Nginx API Gateway** as the single entry point (`http://localhost:8080/key`).

We experimented with dynamic autoscaling by setting Nginx to use Docker's internal DNS resolver (`127.0.0.11`). Because Nginx typically caches DNS resolutions at startup, we configured it dynamically:
```nginx
resolver 127.0.0.11 valid=5s;
location /key {
    set $upstream http://kgs:8080;
    proxy_pass $upstream/key;
}
```
This allows us to seamlessly scale the KGS instances up and down using:
`docker-compose up -d --scale kgs=10`
Nginx detects the new containers within 5 seconds and instantly begins routing traffic to them without needing a restart.

*(Note: We initially experimented with Traefik for dynamic service discovery, but reverted to Nginx due to stubborn `/var/run/docker.sock` permission restrictions on Docker Desktop for Mac).*

### 3. Observability & APM
The cluster includes a local Elastic Stack (version 7.17.13 to support standalone APM without Fleet integration complexity). The Go KGS service is heavily instrumented using Elastic's Go APM Agent. 
Every HTTP request to the `/key` endpoint generates a distributed trace, allowing us to monitor latency, throughput, and error rates via Kibana (`http://localhost:5601`).

---

## Load Testing & Performance Tuning (Target: 1M TPM)

We conducted rigorous stress testing using Apache Bench (`ab`) running concurrently across 12 terminal windows. Our goal was to achieve **1,000,000 Transactions Per Minute (TPM)**, which translates to roughly **16,666 Requests Per Second (RPS)**.

During the load testing, we identified and eliminated several major bottlenecks:

#### Bottleneck 1: Zookeeper Network IO
Initially, the `rangeSize` claimed by each server was `1,000`. At 16K RPS, the KGS instances were synchronously pausing to fetch a new block from ZooKeeper 16 times a second. This caused massive IO blocking.
*   **Fix**: We increased the `rangeSize` to `100,000`. This completely eliminated the ZooKeeper bottleneck, as each container now only needs to talk to ZooKeeper once every ~6 seconds.

#### Bottleneck 2: Nginx Connection Limits
The default Nginx `worker_connections` is capped at `1024`. Our load testing (`ab -c 500` x 12 terminals) generated 6,000 concurrent connections, immediately saturating the gateway.
*   **Fix**: We increased `worker_connections` to `10240`.

#### Bottleneck 3: TCP Handshake Overhead
Nginx was establishing a brand new TCP connection with the backend Go containers for every single proxy request. At 16K RPS, the TCP handshake overhead was severely throttling throughput.
*   **Fix**: We configured an Nginx `upstream` block with a `keepalive 500` setting to maintain 500 idle connections. We also enforced `proxy_http_version 1.1` and stripped the `Connection` header so Nginx successfully reuses connections to the backends.

#### Bottleneck 4: APM Tracing Overhead
Generating and indexing 16,666 distributed traces per second into a local Elasticsearch container quickly becomes a CPU and disk bottleneck. For extreme local load testing, APM tracing should either be temporarily disabled (`ELASTIC_APM_ACTIVE=false`) or aggressively sampled.

---

## How to Run

1.  **Start the Cluster** (with 3 KGS nodes):
    ```bash
    docker-compose up -d --build --scale kgs=3
    ```
2.  **Generate a Key**:
    ```bash
    curl http://localhost:8080/key
    ```
3.  **Scale the Cluster dynamically**:
    ```bash
    docker-compose up -d --scale kgs=10
    ```
4.  **View Traces in Kibana**:
    Navigate to `http://localhost:5601` -> Observability -> APM.
5.  **Stop the Cluster**:
    ```bash
    docker-compose down
    ```
