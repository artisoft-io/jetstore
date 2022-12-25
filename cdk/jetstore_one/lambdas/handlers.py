import json
import urllib3
import urllib
import json
import os


jets_api_host = os.environ['JETS_API_HOST']

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
        "password": os.environ['SYSTEM_PWD']
    })

    http = urllib3.PoolManager()

    try:
        login_r = http.request('POST', jets_api_host + '/login',
                     headers={'Content-Type': 'application/json'},
                     body=encoded_login_body)
    
    
        login_resp_body = login_r.data.decode('utf-8')
        login_resp_dict = json.loads(login_r.data.decode('utf-8'))
        
        print(login_resp_body)
        
        login_token     = login_resp_dict['token']
    except Exception as e:
        print(e)
        print('Error logging in to {}.'.format(jets_api_host))
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
    
    print(encoded_registerFileKey_body)
    
    try:
        registerFileKey_r = http.request('POST', jets_api_host + '/registerFileKey',
                     headers=req_headers,
                     body=encoded_registerFileKey_body)
                     
        registerFileKey_resp_body = registerFileKey_r.data.decode('utf-8')
        registerFileKey_resp_dict = json.loads(registerFileKey_r.data.decode('utf-8'))   
        
        print(registerFileKey_resp_body)
    
    except Exception as e:
        print(e)
        print('Error registering client {}, object type: {}, key'.format(client, object_type, key))
        raise e        
    

    return {
        'statusCode': 200,
        'body': json.dumps('Successfully registered client {}, object type: {}, key'.format(client, object_type, key))
    }