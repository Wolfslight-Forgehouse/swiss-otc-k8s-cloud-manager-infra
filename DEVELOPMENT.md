# Development Guide

## Project Structure

```
swiss-otc-cloud-manager/
├── cmd/
│   └── cloud-controller-manager/
│       └── cloud-controller-manager.go   # Main entry point, registers OTC provider
├── pkg/
│   └── opentelekomcloud/
│       ├── opentelekomcloud.go           # Cloud provider interface implementation
│       ├── config/
│       │   └── config.go                 # YAML config parser (cloud.conf)
│       ├── loadbalancer/
│       │   ├── loadbalancer.go           # K8s LoadBalancer interface (Ensure/Delete/Get)
│       │   └── client.go                 # OTC ELB v3 HTTP client (all API calls)
│       └── instances/
│           └── instances.go              # Node instance metadata (minimal)
├── charts/
│   └── swiss-otc-ccm/                    # Helm chart
├── deploy/
│   └── swiss-otc-ccm.service            # Systemd unit file
├── docs/
│   ├── NETWORKING.md                     # Critical: OTC networking & VXLAN fix
│   └── CHANGELOG.md                     # Release history
├── Dockerfile                            # Multi-stage build
├── README.md                             # Project overview
└── DEVELOPMENT.md                        # This file
```

## API Flow: Create LoadBalancer

```mermaid
sequenceDiagram
    participant K as Kubernetes
    participant CCM as CCM
    participant IAM as OTC IAM
    participant ELB as OTC ELB v3
    participant VPC as OTC VPC

    K->>CCM: Service type:LoadBalancer created
    CCM->>IAM: POST /v3/auth/tokens (project-scoped)
    IAM-->>CCM: Token

    CCM->>ELB: POST /loadbalancers<br/>(ip_target_enable: true)
    ELB-->>CCM: LB ID + VIP

    loop For each service port
        CCM->>ELB: POST /pools (ROUND_ROBIN)
        ELB-->>CCM: Pool ID
        
        CCM->>ELB: POST /listeners (TCP/UDP)
        ELB-->>CCM: Listener ID
        
        loop For each node
            CCM->>ELB: POST /pools/{id}/members<br/>(node IP + NodePort)
        end
        
        CCM->>ELB: POST /healthmonitors<br/>(timeout=10, delay=5)
    end

    CCM->>VPC: GET /ports/{vip_port_id}
    VPC-->>CCM: Security Group ID
    
    loop For each service port
        CCM->>VPC: POST /security-group-rules<br/>(TCP port from allowed CIDRs)
    end

    Note over CCM: If annotation otc.io/eip-bandwidth set:
    CCM->>VPC: POST /publicips (bind to VIP port)

    CCM->>K: Update Service status<br/>(LoadBalancer IP = VIP)
```

## API Flow: Delete LoadBalancer

```mermaid
sequenceDiagram
    participant K as Kubernetes
    participant CCM as CCM
    participant ELB as OTC ELB v3
    participant VPC as OTC VPC

    K->>CCM: Service deleted

    Note over CCM: Delete in dependency order<br/>(OTC returns 409 if deps exist)

    CCM->>ELB: GET /listeners?lb_id=X
    
    loop For each listener
        CCM->>ELB: DELETE /listeners/{id}
    end

    CCM->>ELB: GET /pools?lb_id=X
    
    loop For each pool
        Note over CCM: Members & health monitors<br/>auto-deleted with pool
        CCM->>ELB: DELETE /pools/{id}
    end

    Note over CCM: If EIP was assigned:
    CCM->>VPC: DELETE /publicips/{id}

    CCM->>ELB: DELETE /loadbalancers/{id}
    
    CCM->>K: Remove finalizer
```

## Network Layout

```mermaid
graph TB
    subgraph OTC["Open Telekom Cloud (eu-ch2)"]
        subgraph VPC["VPC 10.0.0.0/16"]
            subgraph Subnet["Subnet 10.0.1.0/24"]
                Bastion[Bastion<br/>10.0.1.64]
                Master[Master<br/>10.0.1.130]
                W1[Worker 1<br/>10.0.1.220]
                W2[Worker 2<br/>10.0.1.236]
                ELB_VIP[ELB VIP<br/>10.0.1.x]
            end
        end
        SG[Security Group<br/>TCP: 22,6443,9345,10250,30000-32767<br/>UDP: 8472 ← CRITICAL]
    end

    subgraph K8s["Kubernetes Overlay (10.42.0.0/16)"]
        Master --- |VXLAN 8472| W1
        Master --- |VXLAN 8472| W2
        W1 --- |VXLAN 8472| W2
    end

    ELB_VIP --> |NodePort| Master
    ELB_VIP --> |NodePort| W1
    ELB_VIP --> |NodePort| W2

    style SG fill:#ff9,stroke:#333,stroke-width:2px
    style ELB_VIP fill:#f96,stroke:#333,stroke-width:2px
```

## Swiss OTC eu-ch2 API Specifics

### Health Monitor: timeout >= delay

Standard OpenStack documentation states `timeout < delay`. **Swiss OTC eu-ch2 requires the opposite:**

```
✅ Working:  delay=5, timeout=10, max_retries=3
❌ Fails:    delay=5, timeout=3, max_retries=3
```

All pre-existing working ELBs in the project use `timeout=10, delay=5`.

### vip_subnet_cidr_id (NOT vip_subnet_id)

The ELB v3 API in eu-ch2 uses `vip_subnet_cidr_id` which maps to the **Neutron subnet ID** (from the VPC subnet API), not `vip_subnet_id`.

### ip_target_enable: true

Required for IP-based backend members. Without this, the ELB creates internal routing ports but does not forward actual traffic to backends.

### VIP Security Group

OTC automatically creates a separate security group on the ELB's VIP port. This SG only has a self-referencing rule + SSH by default. The CCM must add ingress rules for each service port.

### Async Delete (409 Race Condition)

OTC processes listener and pool deletions asynchronously. Attempting to delete the parent ELB before child resources finish deleting returns HTTP 409 (`ELB.8907`). The CCM handles this by deleting in dependency order with waits.

## Building

```bash
# Standard build
go build -o bin/swiss-otc-cloud-controller-manager ./cmd/cloud-controller-manager/

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o bin/swiss-otc-cloud-controller-manager ./cmd/cloud-controller-manager/

# Docker
docker build -t swiss-otc-ccm:latest .
```

## Debugging

### CCM Logs (systemd)
```bash
journalctl -u swiss-otc-ccm -f
```

### Verify ELB via API
```bash
TOKEN=$(curl -s -X POST $IAM_ENDPOINT/auth/tokens ...)
curl -H "X-Auth-Token: $TOKEN" "$ELB_ENDPOINT/loadbalancers" | python3 -m json.tool
```

### Check pool member status
```bash
curl -H "X-Auth-Token: $TOKEN" "$ELB_ENDPOINT/pools/$POOL_ID/members"
# operating_status: ONLINE = healthy, OFFLINE = health check failing
```

### tcpdump for traffic verification
```bash
# On a node: check if ELB health checks arrive
tcpdump -i enp0s3 tcp port $NODEPORT -nn

# Check VXLAN overlay traffic
tcpdump -i enp0s3 udp port 8472 -nn
```
