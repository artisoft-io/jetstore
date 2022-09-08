#!/usr/bin/env python3

import requests
import json
import sys
import os
from absl import app
from absl import flags


dev_mode   = os.getenv('JETSTORE_DEV_MODE')
argo_host  = os.getenv('ARGO_HOST', 'https://localhost:9999')
argo_token = os.getenv('ARGO_TOKEN', '')

FLAGS = flags.FLAGS
flags.DEFINE_string("client", None, "")
flags.DEFINE_boolean("doNotLockSessionId", False, "")
flags.DEFINE_string("groupingColumn", None, "")
flags.DEFINE_string("inFile", None, "")
flags.DEFINE_string("loaderSessionId", None, "")
flags.DEFINE_string("objectType", None, "")
flags.DEFINE_string("peKey", None, "")
flags.DEFINE_string("run_loader", None, "")
flags.DEFINE_string("run_server", None, "")
flags.DEFINE_string("s3InputDirectory", None, "")
flags.DEFINE_string("serverSessionId", None, "")
flags.DEFINE_string("table", None, "")
flags.DEFINE_string("userEmail", None, "")
flags.DEFINE_string("processName", None, "")


payload = { "workflow": {
        "apiVersion": "argoproj.io/v1alpha1",
        "kind": "Workflow",
        "metadata": {
            "generateName": "jetstore-process-workflow-",
            "namespace": "argo"
        },
        "spec": {
            "arguments": {
                "parameters": []
            },
            "workflowTemplateRef": {
                "name": "jetstore-workflow-template"
            }
        }
    }
}

def main(argv):
  del argv  # Unused.

  client              = FLAGS.client
  doNotLockSessionId  = FLAGS.doNotLockSessionId
  groupingColumn      = FLAGS.groupingColumn
  inFile              = FLAGS.inFile
  loaderSessionId     = FLAGS.loaderSessionId
  objectType          = FLAGS.objectType
  peKey               = FLAGS.peKey
  run_loader          = FLAGS.run_loader
  run_server          = FLAGS.run_server
  s3InputDirectory    = FLAGS.s3InputDirectory
  serverSessionId     = FLAGS.serverSessionId
  table               = FLAGS.table
  userEmail           = FLAGS.userEmail
  processName         = FLAGS.processName

  params = []

  if client:
    params.append({"name":"client", "value":client})
  if doNotLockSessionId:
    params.append({"name":"doNotLockSessionId", "value":'-doNotLockSessionId'})
  if groupingColumn:
    params.append({"name":"groupingColumn", "value":groupingColumn})
  if inFile:
    params.append({"name":"inFile", "value":inFile})
  if loaderSessionId:
    params.append({"name":"loaderSessionId", "value":loaderSessionId})
  if objectType:
    params.append({"name":"objectType", "value":objectType})
  if peKey:
    params.append({"name":"peKey", "value":peKey})
  if run_loader:
    params.append({"name":"run_loader", "value":run_loader})
  if run_server:
    params.append({"name":"run_server", "value":run_server})
  if s3InputDirectory:
    params.append({"name":"s3InputDirectory", "value":s3InputDirectory})
  if serverSessionId:
    params.append({"name":"serverSessionId", "value":serverSessionId})
  if table:
    params.append({"name":"table", "value":table})
  if userEmail:
    params.append({"name":"userEmail", "value":userEmail})
  if processName:
    params.append({"name":"processName", "value":processName})


  payload["workflow"]["spec"]["arguments"]["parameters"].extend(params)  

  print(payload)
  
  headers =   {
            'Cookie': 'authorization=Bearer ' + argo_token,
            }

  if not dev_mode:
    try:
      
      print('--------------------PAYLOAD-----------------------')
      print(payload) 
      print('--------------------------------------------------')

      r = requests.post(argo_host + '/api/v1/workflows/argo', verify=False, data=json.dumps(payload), headers=headers)

      print("STATUS IS:" + str(r.status_code))
      print(str(r.json()))
    except Exception as e:
      print(f"Unexpected {e=}, {type(e)=}")
      sys.exit('Error while submitting workflow.')
    
    if str(r.status_code) != '200':
      sys.exit('Error while submitting workflow: Got Status Code: ' + str(r.status_code))

  else:
    print('--------------------PAYLOAD-----------------------')
    print(payload) 
    print('--------------------------------------------------') 
    



if __name__ == '__main__':
  app.run(main)

