# Building the Bottlerocket SDK

If you'd like to build your own images instead of relying on the [published images](https://github.com/bottlerocket-os/bottlerocket-sdk#availability), follow these steps.

## Build the images

### Dependencies

#### System Requirements

The build process images can consume in excess of 40GB in the docker directory.

The build process is also fairly demanding on your CPU since we build all included software from scratch.

#### Linux

The build system requires certain operating system packages to be installed.

Ensure the following OS packages are installed:

##### Ubuntu

```shell
apt install build-essential
```

##### Fedora

```shell
yum install make
```

#### Docker

Bottlerocket uses [Docker](https://docs.docker.com/install/#supported-platforms) for image builds.

You'll need to have Docker installed and running, with your user account added to the `docker` group.
Docker's [post-installation steps for Linux](https://docs.docker.com/install/linux/linux-postinstall/) will walk you through that.

You'll also need to enable `experimental features` by editing Docker's `daemon.json` and setting  `experimental` to `true`.
This is necessary because builds rely on Docker's experimental `squash` feature.

> Note: If you're on a newer Linux distribution using the unified cgroup hierarchy with cgroups v2, you may need to disable it to work with some versions of runc.
> You'll know this is the case if you see an error like `docker: Error response from daemon: OCI runtime create failed: this version of runc doesn't work on cgroups v2: unknown.`
> Set the kernel parameter `systemd.unified_cgroup_hierarchy=0` in your boot configuration (e.g. GRUB) and reboot.

### Build process

To build the images, run:

```shell
make ARCH="my-target-arch-here"
```

One thing to keep in mind is the difference between the container host architecture and target architecture (the `ARCH` argument being for target arch).
If you plan on building for each architecture, you will want to build each target architecture on each host architecture.
In other words, you’ll want to do `make ARCH=x86_64` and `make ARCH=aarch64` on an **x86_64** machine and on an **aarch64** machine for a total of four builds.

## Use your images

To use your custom built SDK and toolchain, you will need to modify the `Makefile.toml` in the top-level of your Bottlerocket OS repo.

Replace `BUILDSYS_SDK_VERSION` and `BUILDSYS_REGISTRY` to match what you have built/published.
If you changed your SDK’s name you will also need also change `BUILDSYS_SDK_IMAGE` and `BUILDSYS_TOOLCHAIN` further down the toml file.

From there, you will be able to [build Bottlerocket](https://github.com/bottlerocket-os/bottlerocket/blob/develop/BUILDING.md).

## Publish your images

To publish your images, we recommend creating separate repositories per target architecture for the SDK and toolchain images like so:

- my-custom-bottlerocket-sdk-aarch64
- my-custom-bottlerocket-sdk-x86_64
- my-custom-bottlerocket-toolchain-aarch64
- my-custom-bottlerocket-toolchain-x86_64

If you'd like to build Bottlerocket on multiple host architectures, we recommend that you publish multi-arch manifests to each of the repositories you created.
See the Amazon ECR documentation for [pushing a multi-architecture image](https://docs.aws.amazon.com/AmazonECR/latest/userguide/docker-push-multi-architecture-image.html).

For easy publishing, we've added a publish target to the Makefile as well as a script `publish-sdk`.

To publish via the `publish-sdk` script, run:

```shell
./publish-sdk \
  --registry="aws_account_id.dkr.ecr.us-east-1.amazonaws.com" \
  --sdk-name="my-custom-bottlerocket" \
  --version="v0.1.0"
```
