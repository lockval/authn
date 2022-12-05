# lockval/authn

You can learn how to develop a login(Service AuthN) and a guest(3rd Platform AuthN)

Simply put, these services are an http server registering with etcd for service discovery.

According to this example, you can use any language or tool to develop the function that meets your needs

### Introduction

Authentication services consist of a Service AuthN and multiple 3rd Platform AuthNs

For example, if you need to support Apple, Google, Microsoft and guest account login, you need the following services to form your login authentication server:

login       (Service AuthN)
apple       (3rd Platform AuthN)
google      (3rd Platform AuthN)
microsoft   (3rd Platform AuthN)
guest       (3rd Platform AuthN)

