# README

## コンテンツ
- スタート前の準備
- ***とにかく急いで構築したい人向け***（terraform を利用）
- アプリケーションのデプロイ
- Google BigQuery へのログの転送
- **オプション**: マネージド証明書付きの Google Cloud LoadBalancer のデプロイ

![architecture_diagram](diagram/egg.png)


## スタート前の準備
### 1. プロジェクトへのサインイン
```
gcloud auth login
gcloud auth application-default login
```
### 2. spanner-cli のインストール  
もし、Go をインストールしていない場合はこちら  
https://go.dev/doc/install
```
go install github.com/cloudspannerecosystem/spanner-cli@latest
export PATH=$PATH:~/go/bin
```
### 3. 環境変数の設定
```
export GOOGLE_CLOUD_PROJECT=プロジェクトID
```

### 4. リポジトリをローカルへ clone
```
git clone https://github.com/shin5ok/egg-architecting
```
---

## とにかく急いで構築したい人向け
1. terraform を[インストール](https://developer.hashicorp.com/terraform/downloads)します

2. terraform の環境を初期化します
```
cd terraform/
terraform init
```

3. 環境変数をセット
ご自分の環境に合わせて準備してください
```
export TF_VAR_project=<Google Cloud のプロジェクトID, 例: my-project-xxxxxx>
export TF_VAR_domain=<アクセスに使いたいFQDN>
export TF_VAR_gcs=<Google Cloud Storageの名前, 例: my-bucket-xxxxxx>
export TF_VAR_region=asia-northeast1
export TF_VAR_zone=asia-northeast1-a
```

3. 実行
自分がリポジトリのトップディレクトリにいることを確認して実行します  
（Makefile と同じディレクトリです）

- 念のため、環境をクリーンアップ
```
make clean
unset SPANNER_EMULATOR_HOST
```
- Google Cloud の環境とアプリケーションをデプロイします
```
make all
```

以上、完了です  
[こちら](#7-おめでとう)まで移動して、テストしましょう

---


## アプリケーションのデプロイ

### 1. gcloud コマンドの環境作成と設定
```
gcloud config configurations create egg-test
gcloud config set project $GOOGLE_CLOUD_PROJECT
```

### 2. 必要な Google Cloud のサービスを有効化
```
gcloud services enable \
spanner.googleapis.com \
run.googleapis.com \
cloudbuild.googleapis.com \
artifactregistry.googleapis.com \
vpcaccess.googleapis.com \
redis.googleapis.com \
```

### 3. Cloud Run サービスで使うサービスアカウントを有効化
```
gcloud iam service-accounts create game-api
```
サービスアカウントへ、Cloud Spanner へのアクセスのための IAM ポリシーを付与
```
export SA=game-api@$GOOGLE_CLOUD_PROJECT.iam.gserviceaccount.com
gcloud projects add-iam-policy-binding $GOOGLE_CLOUD_PROJECT --member=serviceAccount:$SA --role=roles/spanner.databaseUser
```
### 5. Redis を設置するための VPC を構築
```
gcloud compute networks create my-network --subnet-mode=custom
gcloud compute networks subnets create --network=my-network --region=asia-northeast1 --range=10.0.0.0/16 tokyo
```

### 6. Memorystore for Redis を作成
```
gcloud redis instances create test-redis --zone=asia-northeast1-b --network=my-network --region=asia-northeast1
```

### 4. Cloud Spanner インスタンスを作成
```
gcloud spanner instances create --nodes=1 test-instance --description="for production" --config=regional-asia-northeast1
```

### 5. Cloud Spanner インスタンスに、データベースとスキーマを作成して、初期データを登録

#### データベースを準備
```
gcloud spanner databases create --instance test-instance game
```
#### スキーマを作成し、初期データを登録
```
for schema in ./schemas/*ddl.sql ./schemas/*dml.sql;
do
    spanner-cli -p $GOOGLE_CLOUD_PROJECT -i test-instance -d game < $schema
done
```

  spanner-cli を利用して、スキーマとデータを確認します
```
spanner-cli -i test-instance -p $GOOGLE_CLOUD_PROJECT -d game
```
#### コマンド例
```
show tables;
show create table users;
show create table users_items;
show create table items;
select * from items;
```

### 6. Configure a Serverless Access Connector.
```
gcloud compute networks vpc-access connectors create game-api-vpc-access --network my-network --region asia-northeast1 --range 10.8.0.0/28
```

### 7. Cloud Run サービスをデプロイ
アプリケーションで利用する環境変数を設定
```
VA=projects/$GOOGLE_CLOUD_PROJECT/locations/asia-northeast1/connectors/game-api-vpc-access
REDIS_HOST=$(gcloud redis instances describe test-redis --region=asia-northeast1 --format=json | jq .host -r)
SPANNER_STRING=projects/$GOOGLE_CLOUD_PROJECT/instances/test-instance/databases/game
```

#### ***オプション1***: ***buildpacks*** を利用
Dockerfile なしで、コンテナを自動ビルド、Cloud Run にデプロイ  
手順が少なく自動で最適化されたコンテナをデプロイできますが、オプション2に比べて、時間がかかります
```
gcloud run deploy game-api --allow-unauthenticated --region=asia-northeast1 \
--set-env-vars=SPANNER_STRING=$SPANNER_STRING \
--service-account=$SA --source=.
```
以上  

8 に進みます
#### ***オプション2***: 従来どおり Dockerfile を利用
Artifact Registry へのリポジトリの作成と、それを利用する準備
```
gcloud artifacts repositories create my-app --repository-format=docker --location=asia-northeast1
gcloud auth configure-docker asia-northeast1-docker.pkg.dev
```
#### コンテナのビルド
```
IMAGE=asia-northeast1-docker.pkg.dev/$GOOGLE_CLOUD_PROJECT/my-app/game-api
docker build -t game-api -f Dockerfile.option2 .
docker tag game-api $IMAGE
docker push $IMAGE
```
#### Cloud Run サービスへのデプロイ
```
gcloud run deploy game-api --allow-unauthenticated --region=asia-northeast1 \
--set-env-vars=SPANNER_STRING=$SPANNER_STRING \
--service-account=$SA --image $IMAGE
```

### 8. おめでとう!!  
テストしましょう  
Cloud Run サービスに割り当てられた URL は以下のような文字列になります  
"https://game-api-xxxxxxxxx-xx.a.run.app".

テストのため、変数に URL をセット  
（以下は例です）
```
URL="https://game-api-xxxxxxxxx-xx.a.run.app"
```

- ユーザーを作成
```
curl $URL/api/user/foo -X POST
```
レスポンスに生成された ID が含まれています  
形式は UUIDv4 です
- ユーザーにアイテムの追加（購入）
```
USER_ID=<your user id>
ITEM_ID=d169f397-ba3f-413b-bc3c-a465576ef06e
curl $URL/api/user_id/$USER_ID/$ITEM_ID -X PUT
```

- ユーザーが購入したアイテムのリストを取得
```
curl $URL/api/user_id/$USER_ID -X GET
```

## Google BigQuery へのログの転送

### 1. ログの転送先として、BigQuery へデータセットを作成
```
bq mk --location asia-northeast1 dataset1
```

### 2. ログシンクを作成します
```
gcloud logging sinks create game-api-sink \
bigquery.googleapis.com/projects/$GOOGLE_CLOUD_PROJECT/datasets/dataset1 \
--description="for Cloud Run service 'game-api'" \
--log-filter='resource.type="cloud_run_revision" AND resource.labels.configuration_name="game-api" AND jsonPayload.message!=""'
```

### 3. ログシンクが利用する サービスアカウントへ、BigQuery への書き込み権限（BigQuery dataEditor）を付与
```
LOGSA=$(gcloud logging sinks describe game-api-sink --format=json | jq .writerIdentity -r)

gcloud projects add-iam-policy-binding $PROJECT_ID --member=$LOGSA --role=roles/bigquery.dataEditor
```

以上で完了です  
転送されたログを BigQuery テーブルで確認しましょう  
もしかすると、転送が始まるまで、数分待つ必要があるかもしれません



## **オプション**: マネージド証明書付きの Google Cloud LoadBalancer のデプロイ
### 注意: このステップに必要な要件
カスタムドメインの利用のため、あなたが権限をもつドメインのゾーンが必要です

### 1. 外部 IP アドレスの予約
```
gcloud compute addresses create game-api-ip \
    --network-tier=PREMIUM \
    --ip-version=IPV4 \
    --global
```

### 2. Cloud Run サービスを、ロードバランサーのターゲットにするための サーバーレス NEG を準備
```
gcloud compute network-endpoint-groups create game-api \
    --region=asia-northeast1 \
    --network-endpoint-type=serverless  \
    --cloud-run-service=game-api
```

### 3. ロードバランサーのバックエンドサービスを作成
```
gcloud compute backend-services create backend-for-game-api \
    --load-balancing-scheme=EXTERNAL \
    --global
```
これに先程準備したサーバーレス NEG を追加
```
gcloud compute backend-services add-backend backend-for-game-api \
    --global \
    --network-endpoint-group=game-api \
    --network-endpoint-group-region=asia-northeast1
```

### 4. URL マップを作成  
デフォルトの転送先として、先ほど作成したバックエンドサービスを指定
```
gcloud compute url-maps create urlmap-for-game-api \
   --default-service backend-for-game-api
```

### 5. マネージド証明書の作成
```
FQDN=<your FQDN you want to use>
gcloud compute ssl-certificates create ssl-cert-for-game-api \
   --domains $FQDN
```

### 6. ターゲット Proxy を作成
```
gcloud compute target-https-proxies create target-proxy-for-game-api \
   --ssl-certificates=ssl-cert-for-game-api \
   --url-map=urlmap-for-game-api
```

### 7. 転送ルールを作成
```
gcloud compute forwarding-rules create forwarding-to-game-api \
    --load-balancing-scheme=EXTERNAL \
    --network-tier=PREMIUM \
    --address=game-api-ip \
    --target-https-proxy=target-proxy-for-game-api \
    --global \
    --ports=443
```

### 8. DNS レコードをアップデート  
ロードバランサーが利用している IP アドレスを抽出  
（確保した外部 IP アドレスと同じになるはず）

```
gcloud compute addresses describe game-api-ip --global --format=json | jq .address -r
```
この IP アドレスと、FQDNが一致するように、カスタムドメインの DNS レコードを書き換え、反映  
方法は、DNS サーバーや、そのサービスを提供するプロバイダーに依存します

もし Cloud DNS を使っている場合は、下記のようなコマンドで管理ゾーンを作成し、上記の IP アドレスに対応するレコードを追加できます
```
gcloud dns managed-zones create <your-zone-name> --dns-name=<your-domain-name> --description="My Domain"
gcloud dns record-sets create --type=A --zone=<your-zone-name> --rrdatas=<IP address> $FQDN
```
作成したドメインの NS レコードを、上位の権威 DNS に登録することを忘れないでください  
この NS レコードは、こちらのコマンドで取得できます
```
gcloud dns managed-zones describe <your-zone-name> --format=json | jq -r .nameServers[]
```
複数の NS レコードすべてを登録します

証明書が有効になるまで、しばらくかかります（通常 10分以上）  
それまではアクセスしても、4xx/5xx レコードが返却されたり、SSL エラーになります

