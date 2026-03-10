# Swiss OTC RKE2 Architecture

## Network Design

```mermaid
graph TD
    subgraph Internet
        User[User / Browser]
        CI[GitHub Actions]
    end

    subgraph OTC["Swiss OTC eu-ch2"]
        EIP_LB[ELB EIP<br/>185.153.x.x]
        EIP_Bastion[Bastion EIP]
        
        subgraph VPC["VPC 10.0.0.0/16"]
            ELB[ELB v3 Dedicated<br/>Listener → Pool → Members]
            Bastion[Bastion Host<br/>TinyProxy, kubectl, helm<br/>GH Actions Runner]
            
            subgraph RKE2["RKE2 Cluster v1.34.4"]
                Master[Master<br/>API Server, etcd<br/>Control Plane]
                Worker1[Worker 1]
                Worker2[Worker 2]
            end
        end
    end

    User -->|HTTP :80| EIP_LB --> ELB --> Worker1 & Worker2
    CI -->|SSH| EIP_Bastion --> Bastion
    Bastion -->|kubectl| Master
    Bastion -->|TinyProxy :3128| Internet
    Master --> Worker1 & Worker2
```

## Component Stack

```mermaid
graph LR
    subgraph Infrastructure["Terraform Modules"]
        Net[networking<br/>VPC, Subnet, SG, Keypair]
        Jump[jumpserver<br/>Bastion, TinyProxy<br/>GH Runner, kubectl]
        Compute[compute<br/>RKE2 Master + Workers<br/>Cilium CNI]
    end

    subgraph Kubernetes["Kubernetes Stack"]
        RKE2[RKE2 v1.34.4]
        Cilium[Cilium CNI<br/>+ Hubble]
        CCM[Swiss OTC CCM<br/>ELB v3 Controller]
        Ingress[NGINX Ingress]
        CoreDNS[CoreDNS]
    end

    subgraph OTC_APIs["Swiss OTC APIs"]
        ELB_API[ELB v3 API]
        VPC_API[VPC API]
        IAM[IAM / AK-SK Auth]
    end

    Net --> Jump --> Compute
    RKE2 --> Cilium --> CCM
    CCM -->|SDK-HMAC-SHA256| ELB_API & VPC_API
```

## Authentication Flow

```mermaid
sequenceDiagram
    participant K8s as K8s Service Controller
    participant CCM as Swiss OTC CCM
    participant API as OTC API Gateway

    K8s->>CCM: Service type:LoadBalancer created
    CCM->>CCM: Build canonical request
    CCM->>CCM: Sign with HMAC-SHA256(SK, StringToSign)
    CCM->>API: GET /v3/{project}/elb/loadbalancers<br/>Authorization: SDK-HMAC-SHA256 ...
    API->>API: Verify signature
    API-->>CCM: 200 OK / 404 Not Found
    CCM->>API: POST /v3/{project}/elb/loadbalancers<br/>(with EIP if annotated)
    API-->>CCM: 201 Created {id, vip, eips}
    CCM->>API: POST pools, listeners, members, healthmonitor
    CCM->>API: POST security-group-rules
    CCM->>K8s: Patch Service status (EXTERNAL-IP)
```

## GitOps Pipeline

```mermaid
graph LR
    Push[git push] -->|auto| Plan[Terraform Plan]
    Plan -->|manual APPLY| Apply[Terraform Apply<br/>27 resources]
    Apply -->|cloud-init| Runner[GH Runner<br/>auto-registers]
    
    Code[Code Change] -->|manual| Build[CCM Build<br/>Go test → Docker → GHCR]
    Build -->|manual| Deploy[CCM Deploy<br/>Helm on self-hosted runner]
    
    Nuke[manual DESTROY] --> Destroy[Terraform Destroy]
```

## Key Design Decisions

| Decision | Rationale |
|---|---|
| AK/SK over password auth | Stateless, no token lifecycle, CI/CD best practice |
| Self-hosted runner on bastion | Direct cluster access, no SSH tunnels needed |
| Runner bootstraps via cloud-init | Fully automated, no manual setup after apply |
| TinyProxy over NAT Gateway | IAM user lacks NAT Admin permissions |
| ELB v3 from scratch | Swiss OTC eu-ch2 only supports v3 Dedicated, no existing CCM |
| Separate subnet IDs in config | OTC requires Neutron subnet ID ≠ VPC subnet ID |
| Mermaid diagrams | Native GitHub rendering, no external tools |
