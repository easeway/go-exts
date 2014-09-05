Simple Extension Framework for Go
=================================

Go doesn't support dynamic libraries. 
We have to put source code together even the design is extensible. It is impossible to add an extension at runtime.
This framework helps to make Go application extensible without rebuilding from source code, 
and the extensions can be loaded dynamically at runtime.

The idea is simple, run the extension in a separate process and communicate through stdin/stdout.
The protocol is as simple as single-line json. This mechanism is inspired by Node.js IPC protocol.

So you can develop any extension in any language, not limited to Go. Even the host application can be in any language.

Communication Protocol
======================
Each extension is an executable and launched by the host application with stdin/stdout redirected for communication.
If the extension fails to start, the host application will re-try for a few times and finally give up.
The message is composed as a single-line json string, with predefined outer structure.

The current version supports bi-directional events which means the host application can send events to extension, 
and extension is also able to send events to host. 
But only single-direction invocation from host to extension.
In future, it is possible to extend the framework for invocation from extension to host.

Wire format of IPC protocol
===========================
Put the json string in a single-line (terminated by `'\n'`)

```json
{
    "event": "<TYPE>",
    "id":    "<REQUEST-ID>",
    "name":  "<EVENT-NAME>",
    "data":  EVENT-DATA,
    "error": "<ERROR-MSG>"
}

```

- `TYPE`: currently defined as `E` for event, `R` for request/reply
- `REQUEST-ID`: any unique string used for request (`TYPE` is `R`), reply should put the same id back
- `EVENT-NAME`: application defined event name or action name (for requests)
- `EVENT-DATA`: nested json object, application specific
- `ERROR-MSG`: replies uses this field for simple error message.
