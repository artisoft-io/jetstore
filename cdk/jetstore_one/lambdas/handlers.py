import json
import urllib3
import urllib
import base64
import os

import boto3
from botocore.exceptions import ClientError


def get_secret():
    """
    Gets the secret
    """
    # Get env vars we passed in the stack.
    secret_name = os.getenv("SYSTEM_PWD_SECRET")
    region_name = os.getenv("JETS_REGION")

    # Create a Secrets Manager client
    session = boto3.session.Session()
    client = session.client(
        service_name="secretsmanager", region_name=region_name
    )

    # In this sample we only handle the specific exceptions for the 'GetSecretValue' API.
    # See https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_GetSecretValue.html
    # We rethrow the exception by default.

    try:
        # We need only the name of the secret
        get_secret_value_response = client.get_secret_value(
            SecretId=secret_name
        )
    except ClientError as e:
        if e.response["Error"]["Code"] == "DecryptionFailureException":
            # Secrets Manager can't decrypt the protected secret text using the provided KMS key.
            # Deal with the exception here, and/or rethrow at your discretion.
            raise e
        elif e.response["Error"]["Code"] == "InternalServiceErrorException":
            # An error occurred on the server side.
            # Deal with the exception here, and/or rethrow at your discretion.
            raise e
        elif e.response["Error"]["Code"] == "InvalidParameterException":
            # You provided an invalid value for a parameter.
            # Deal with the exception here, and/or rethrow at your discretion.
            raise e
        elif e.response["Error"]["Code"] == "InvalidRequestException":
            # You provided a parameter value that is not valid for the current state of the resource.
            # Deal with the exception here, and/or rethrow at your discretion.
            raise e
        elif e.response["Error"]["Code"] == "ResourceNotFoundException":
            # We can't find the resource that you asked for.
            # Deal with the exception here, and/or rethrow at your discretion.
            raise e
        else:
            raise e
    else:
        # Decrypts secret using the associated KMS key.
        # Depending on whether the secret is a string or binary, one of these fields will be populated.
        if "SecretString" in get_secret_value_response:
            secret = get_secret_value_response["SecretString"]
            # If you have multiple secret values, you will need to json.loads(secret) here and then access the values using dict keys
            return secret
        else:
            decoded_binary_secret = base64.b64decode(
                get_secret_value_response["SecretBinary"]
            )
            return decoded_binary_secret


jets_api_url = os.environ['JETS_API_URL']

def register_key(event, context):
    
    print('register_key called with event:',str(event))

    # extract s3 key from event notification
    key = urllib.parse.unquote_plus(event['Records'][0]['s3']['object']['key'], encoding='utf-8')    
    
    # split key into components and partitions to extract client and object_type
    key_components = key.split('/')
    
    key_partition_dict = {}
    for c in key_components:
        if "=" in c:
            partition = c.partition('=')
            key_partition_dict[partition[0]] = partition[2]

    print('Extracted from event:',key_partition_dict)    
    
    client      = key_partition_dict.get('client')
    object_type = key_partition_dict.get('object_type')
    if(client is None or object_type is None):
      print('Invalid event, bailing out!')
      return {
        'statusCode': 200,
        'body': json.dumps('Invalid key: {}'.format(key))
      }
    
    # login to Jets API and retrieve token
    encoded_login_body = json.dumps({
        "user_email": os.environ['SYSTEM_USER'],
        "password": get_secret()
    })

    http = urllib3.PoolManager()

    try:
        print("Calling",jets_api_url,"/login")
        login_r = http.request('POST', jets_api_url + '/login',
                     headers={'Content-Type': 'application/json'},
                     body=encoded_login_body)
    
    
        login_resp_body = login_r.data.decode('utf-8')
        login_resp_dict = json.loads(login_r.data.decode('utf-8'))
        
        print("Got Response:", login_resp_body)
        
        login_token     = login_resp_dict['token']
    except Exception as e:
        print(e)
        print('Error logging in to {}.'.format(jets_api_url))
        raise e        


    # register s3 key, client name and object type with Jets API 
    
    req_headers = {
    'Content-Type': 'application/json',
    'Authorization': 'token ' + login_token,
    }

    encoded_registerFileKey_body = json.dumps({
        "action": "register_keys",
        "data":[{"client":client, "object_type":object_type, "file_key":key}]
        
    })
    
    print("registerKey request:",encoded_registerFileKey_body)
    
    try:
        registerFileKey_r = http.request('POST', jets_api_url + '/registerFileKey',
                     headers=req_headers,
                     body=encoded_registerFileKey_body)
                     
        registerFileKey_resp_body = registerFileKey_r.data.decode('utf-8')
        registerFileKey_resp_dict = json.loads(registerFileKey_r.data.decode('utf-8'))   
        
        print("registerKey response:",registerFileKey_resp_body)
    
    except Exception as e:
        print(e)
        print('Error registering client {}, object type: {}, key'.format(client, object_type, key))
        raise e        
    

    return {
        'statusCode': 200,
        'body': json.dumps('Successfully registered client {}, object type: {}, key'.format(client, object_type, key))
    }