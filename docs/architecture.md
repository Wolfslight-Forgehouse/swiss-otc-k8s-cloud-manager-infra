# Architecture — RKE2 auf Swiss OTC mit Cloud Manager

## Überblick

RKE2 Kubernetes Cluster auf der Swiss Open Telekom Cloud (Region eu-ch2) mit automatischem ELB-Management via OTC Cloud Controller Manager.

## Komponenten

```mermaid
graph TD
    subgraph GitHub
        GH_PR[Pull Request] -->|trigger| CI[CI: Lint & Validate]
        GH_PUSH[Push to main] -->|trigger| DEPLOY[Deploy Workflow]
        GH_MANUAL[Manual Dispatch] -->|trigger| DEPLOY
    end

    subgraph Swiss_OTC["Swiss OTC (eu-ch2)"]
        subgraph VPC["VPC 10.0.0.0/16"]
            subgraph Subnet["Subnet 10.0.1.0/24"]
                MASTER[RKE2 Master\ns3.xlarge.4]
                WORKER1[RKE2 Worker 1\ns3.large.4]
                WORKER2[RKE2 Worker 2\ns3.large.4]
            end
            ELB[Elastic Load Balancer]
        end
        FIP[Floating IP] --> MASTER
        ELB --> WORKER1
        ELB --> WORKER2
        OBS[OTC Object Storage\nTerraform State]
    end

    subgraph Kubernetes["Kubernetes / kube-system"]
        CCM[OTC Cloud Controller Manager]
        SVC[Service type:LoadBalancer]
    end

    DEPLOY -->|terraform apply| Swiss_OTC
    DEPLOY -->|helm install| CCM
    SVC -->|creates| ELB
    CCM -->|manages| ELB
    CCM -->|manages| FIP
```

## Deployment Flow

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant GH as GitHub Actions
    participant TF as Terraform
    participant OTC as Swiss OTC
    participant K8s as Kubernetes

    Dev->>GH: Push to main / Manual Dispatch
    GH->>TF: terraform init + apply
    TF->>OTC: Create VPC, Subnet, Security Groups
    TF->>OTC: Create ECS Instances (1 Master + 2 Worker)
    OTC->>OTC: RKE2 Server starts (user-data)
    GH->>K8s: Fetch kubeconfig
    GH->>K8s: helm upgrade --install otc-cloud-manager
    K8s->>OTC: Cloud Manager authenticates via IAM
    
    Note over K8s,OTC: ELB Automation aktiv
    Dev->>K8s: kubectl apply Service type:LoadBalancer
    K8s->>OTC: Cloud Manager creates ELB
    OTC-->>K8s: Floating IP assigned
    K8s-->>Dev: EXTERNAL-IP verfügbar
```

## Netzwerk-Topologie

```mermaid
graph LR
    Internet((Internet)) --> FIP_MASTER[Floating IP\nMaster]
    Internet --> ELB[ELB\nFloating IP]
    
    subgraph OTC_VPC["OTC VPC 10.0.0.0/16"]
        FIP_MASTER --> MASTER["Master\n10.0.1.10"]
        ROUTER[Router] --> MASTER
        ROUTER --> WORKER1["Worker 1\n10.0.1.11"]
        ROUTER --> WORKER2["Worker 2\n10.0.1.12"]
        ELB --> WORKER1
        ELB --> WORKER2
    end
```
