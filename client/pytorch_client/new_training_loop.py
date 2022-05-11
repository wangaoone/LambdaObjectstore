"""
Deep learning training cycle.
"""
from __future__ import annotations

import random
import time
import os

import numpy as np
import torch
import torch.nn as nn
from torch.utils.data import DataLoader

import cnn_models
import logging_utils
import updated_datasets
import utils

LOGGER = logging_utils.initialize_logger()

SEED = 1234
NUM_EPOCHS = 10
DEVICE = "cuda:0"
LEARNING_RATE = 1e-3

random.seed(SEED)
torch.manual_seed(SEED)
random.seed(SEED)
np.random.seed(SEED)


def training_cycle(
    model: nn.Module,
    train_dataloader: DataLoader,
    test_dataloader: DataLoader,
    optim_func: torch.optim.Adam,
    loss_fn: nn.CrossEntropyLoss,
    num_epochs: int = 1,
    device: str = "cuda:0",
):
    start_time = time.time()
    num_batches = len(train_dataloader)
    for epoch in range(num_epochs):
        iteration = 0
        running_loss = 0.0
        model.train()
        for idx, (images, labels) in enumerate(train_dataloader):
            images = images.to(device)
            labels = labels.to(device)
            logits, _ = model(images)
            loss = loss_fn(logits, labels)

            iteration += 1
            optim_func.zero_grad()
            loss.backward()
            optim_func.step()
            running_loss += float(loss.item())

            if not idx % 100:
                print(
                    (
                        f"Epoch: {epoch+1:03d}/{num_epochs:03d} |"
                        f" Batch: {idx+1:03d}/{num_batches:03d} |"
                        f" Cost: {running_loss/iteration:.4f}"
                    )
                )
                iteration = 0
                running_loss = 0.0

        total_time_taken = sum(train_dataloader.dataset.load_times)
        LOGGER.info(
            "Finished Epoch %d for %s. Total load time for %d samples is %.3f sec.",
            epoch + 1,
            str(train_dataloader.dataset),
            train_dataloader.dataset.total_samples,
            total_time_taken,
        )

        LOGGER.info("Running Testing:")
        with torch.no_grad():
            num_correct = 0
            test_samples = 0
            for idx, (images, labels) in enumerate(test_dataloader):
                images = images.to(device)
                labels = labels.to(device)
                logits, _ = model(images)
                logit_preds = torch.argmax(logits, axis=1)
                num_correct += torch.sum(logit_preds == labels)
                test_samples += labels.shape[0]

            total_time_taken = sum(test_dataloader.dataset.load_times)
            perc_correct = num_correct / test_samples * 100

            LOGGER.info(
                "Finished Testing Epoch %d for %s. Total load time for %d samples is %.3f sec.",
                epoch + 1,
                str(test_dataloader.dataset),
                test_dataloader.dataset.total_samples,
                total_time_taken,
            )
            LOGGER.info(
                "Test results: %d / %d = %f",
                num_correct,
                test_samples,
                perc_correct,
            )

    end_time = time.time()
    print(f"Time taken: {end_time - start_time}")


def initialize_model(
    model_type: str, num_channels: int, device: str = "cuda:0"
) -> tuple[nn.Module, nn.CrossEntropyLoss, torch.optim.Adam]:
    if model_type == "resnet":
        print("Initializing Resnet50 model")
        model = cnn_models.Resnet50(num_channels)
    elif model_type == "efficientnet":
        print("Initializing EfficientNetB4 model")
        model = cnn_models.EfficientNetB4(num_channels)
    elif model_type == "densenet":
        print("Initializing DenseNet161 model")
        model = cnn_models.DenseNet161(num_channels)
    else:
        print("Initializing BasicCNN model")
        model = cnn_models.BasicCNN(num_channels)

    model = model.to(device)
    model.train()
    loss_fn = nn.CrossEntropyLoss()
    optim_func = torch.optim.Adam(model.parameters(), lr=LEARNING_RATE)
    return model, loss_fn, optim_func


if __name__ == "__main__":
    LOGGER.info("TRAINING STARTED")

    # CIFAR DISK
    train_cifar, test_cifar = utils.split_cifar_data(
        "/data/local/cifar_images",
        os.path.join(os.path.dirname(__file__), "cifar_test_fnames.txt"),
    )

    normalize_cifar = utils.normalize_image(True)

    train_ds = updated_datasets.DatasetDisk(train_cifar, 0, dataset_name="CIFAR-10", img_transform=normalize_cifar)
    test_ds = updated_datasets.DatasetDisk(test_cifar, 0, dataset_name="CIFAR-10", img_transform=normalize_cifar)

    train_dataloader = DataLoader(train_ds, batch_size=32, shuffle=True, num_workers=0, pin_memory=True)
    test_dataloader = DataLoader(test_ds, batch_size=32, shuffle=True, num_workers=0, pin_memory=True)

    model, loss_fn, optim_func = initialize_model("resnet", num_channels=3)
    print("Running training with the EBS dataloader")
    training_cycle(model, train_dataloader, test_dataloader, optim_func, loss_fn, 3, DEVICE)

    # CIFAR INFINICACHE
    train_infini = updated_datasets.MiniObjDataset(
        "infinicache-cifar-train",
        label_idx=0,
        channels=True,
        dataset_name="CIFAR-10",
        img_dims=(3, 32, 32),
        obj_size=16,
        img_transform=normalize_cifar,
    )

    test_infini = updated_datasets.MiniObjDataset(
        "infinicache-cifar-test",
        label_idx=0,
        channels=True,
        dataset_name="CIFAR-10",
        img_dims=(3, 32, 32),
        obj_size=16,
        img_transform=normalize_cifar,
    )

    train_infini.initial_set_all_data()
    test_infini.initial_set_all_data()

    # batch_size input should be desired batch size divided by object size
    # E.g., If a batch size of 32 is desired and the object size is 8, then batch_size=4 is the input
    train_infini_dataloader = DataLoader(
        train_infini, batch_size=4, shuffle=True, num_workers=0, collate_fn=utils.infinicache_collate, pin_memory=True
    )

    test_infini_dataloader = DataLoader(
        test_infini, batch_size=4, shuffle=True, num_workers=0, collate_fn=utils.infinicache_collate, pin_memory=True
    )

    model, loss_fn, optim_func = initialize_model("resnet", num_channels=3)
    print("Running training with the InfiniCache dataloader")
    training_cycle(model, train_infini_dataloader, test_infini_dataloader, optim_func, loss_fn, 3, DEVICE)
