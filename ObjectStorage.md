ObjectStorage memory management rules:

* Every function returning a CachedObject (or a collection of objects) returns that object with a +1 count. After using the object call Release

* Functions taking a CachedObject as a parameter should call RegisterConsumer on the object and later call Release.

* When pushing a CachedObject to a WorkerPool always call RegisterConsumer before submitting.
* A WorkerPool taking CachedObjects as a parameter should extract the CachedObject from the params, pass it to the function inside the Task handler and then call Release.